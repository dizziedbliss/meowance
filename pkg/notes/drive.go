package notes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const folderMIME = "application/vnd.google-apps.folder"

// DriveConfig is the Google OAuth client and the refresh token of the account
// whose Drive holds the notes. They come from env, never from the repo.
type DriveConfig struct {
	ClientID, ClientSecret, RefreshToken string
	Root                                 string // top folder, default "meowance"
}

// Drive stores notes in Google Drive under <Root>/notes/<dept>/<sem>/<course>/.
// It uses the narrow drive.file scope, so it can only see what it created.
type Drive struct {
	cfg                           DriveConfig
	http                          *http.Client
	tokenURL, apiBase, uploadBase string
	now                           func() time.Time

	tmu    sync.Mutex // guards the cached access token
	tok    string
	tokExp time.Time

	fmu     sync.Mutex // serialises folder lookups/creates inside one instance
	folders map[string]string
}

// NewDrive builds a store for the given account.
func NewDrive(cfg DriveConfig) *Drive {
	if cfg.Root == "" {
		cfg.Root = "meowance"
	}
	return &Drive{
		cfg:        cfg,
		http:       &http.Client{Timeout: 20 * time.Second},
		tokenURL:   "https://oauth2.googleapis.com/token",
		apiBase:    "https://www.googleapis.com",
		uploadBase: "https://www.googleapis.com",
		now:        time.Now,
		folders:    map[string]string{},
	}
}

// DriveFromEnv reads GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and GOOGLE_REFRESH_TOKEN;
// it returns nil when any is missing, so the notes feature can switch itself off.
func DriveFromEnv() *Drive {
	cfg := DriveConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RefreshToken: os.Getenv("GOOGLE_REFRESH_TOKEN"),
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RefreshToken == "" {
		return nil
	}
	return NewDrive(cfg)
}

func (d *Drive) NewUpload(ctx context.Context, u Upload) (string, error) {
	if err := u.check(); err != nil {
		return "", err
	}
	if u.Origin == "" {
		return "", errBad("origin")
	}
	parent, err := d.folder(ctx, u)
	if err != nil {
		return "", err
	}
	resp, err := d.do(ctx, "POST", d.uploadBase+"/upload/drive/v3/files?uploadType=resumable",
		map[string]any{"name": u.fileName(), "parents": []string{parent}}, map[string]string{
			"X-Upload-Content-Type":   u.MIME,
			"X-Upload-Content-Length": strconv.FormatInt(u.Size, 10),
			"Origin":                  u.Origin, // lets the browser PUT to the session (CORS)
		})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := okStatus(resp); err != nil {
		return "", err
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("notes: drive gave no upload session")
	}
	return loc, nil
}

func (d *Drive) Finish(ctx context.Context, u Upload, key string) (Stored, error) {
	if err := u.check(); err != nil {
		return Stored{}, err
	}
	if !keyPattern.MatchString(key) {
		return Stored{}, ErrRejected
	}
	want, err := d.folder(ctx, u)
	if err != nil {
		return Stored{}, err
	}
	resp, err := d.do(ctx, "GET", d.apiBase+"/drive/v3/files/"+url.PathEscape(key)+"?fields=id,name,size,mimeType,parents", nil, nil)
	if err != nil {
		return Stored{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Stored{}, ErrNotFound
	}
	if err := okStatus(resp); err != nil {
		return Stored{}, err
	}
	var f struct {
		ID, Name, MimeType, Size string
		Parents                  []string
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&f); err != nil {
		return Stored{}, err
	}
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	if f.ID != key || f.Name != u.fileName() || f.MimeType != u.MIME || len(f.Parents) != 1 || f.Parents[0] != want || size != u.Size {
		return Stored{}, ErrRejected
	}
	pr, err := d.do(ctx, "POST", d.apiBase+"/drive/v3/files/"+url.PathEscape(key)+"/permissions",
		map[string]string{"role": "reader", "type": "anyone"}, nil)
	if err != nil {
		return Stored{}, err
	}
	defer pr.Body.Close()
	if err := okStatus(pr); err != nil {
		return Stored{}, err
	}
	return Stored{Key: key, Size: size, MIME: f.MimeType}, nil
}

// Discard deletes the stray file a rejected upload left in its course folder. It looks
// the file up by the note's own name (server-made), never by an id the client sent.
func (d *Drive) Discard(ctx context.Context, u Upload) error {
	if err := u.check(); err != nil {
		return err
	}
	parent, err := d.folder(ctx, u)
	if err != nil {
		return err
	}
	q := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", u.fileName(), parent)
	resp, err := d.do(ctx, "GET", d.apiBase+"/drive/v3/files?pageSize=10&spaces=drive&fields="+url.QueryEscape("files(id)")+"&q="+url.QueryEscape(q), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := okStatus(resp); err != nil {
		return err
	}
	var found struct{ Files []struct{ ID string } }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&found); err != nil {
		return err
	}
	for _, f := range found.Files {
		if err := d.Delete(ctx, f.ID); err != nil {
			return err
		}
	}
	return nil
}

func (d *Drive) Link(key string) string {
	return "https://drive.google.com/uc?id=" + url.QueryEscape(key) + "&export=download"
}

func (d *Drive) Delete(ctx context.Context, key string) error {
	if !keyPattern.MatchString(key) {
		return ErrRejected
	}
	resp, err := d.do(ctx, "DELETE", d.apiBase+"/drive/v3/files/"+url.PathEscape(key), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return okStatus(resp)
}

// folder returns the id of <root>/notes/<dept>/<sem>/<course>, making any missing
// level. A cold instance finds existing folders by name instead of remaking them.
func (d *Drive) folder(ctx context.Context, u Upload) (string, error) {
	d.fmu.Lock()
	defer d.fmu.Unlock()
	parent, path := "root", ""
	for _, name := range []string{d.cfg.Root, "notes", u.Dept, u.Sem, u.Code} {
		path += "/" + name
		if id, ok := d.folders[path]; ok {
			parent = id
			continue
		}
		id, err := d.findOrMake(ctx, name, parent)
		if err != nil {
			return "", err
		}
		d.folders[path] = id
		parent = id
	}
	return parent, nil
}

func (d *Drive) findOrMake(ctx context.Context, name, parent string) (string, error) {
	q := fmt.Sprintf("name='%s' and mimeType='%s' and '%s' in parents and trashed=false", name, folderMIME, parent)
	resp, err := d.do(ctx, "GET", d.apiBase+"/drive/v3/files?pageSize=1&spaces=drive&fields="+url.QueryEscape("files(id)")+"&q="+url.QueryEscape(q), nil, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := okStatus(resp); err != nil {
		return "", err
	}
	var found struct{ Files []struct{ ID string } }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&found); err != nil {
		return "", err
	}
	if len(found.Files) > 0 {
		return found.Files[0].ID, nil
	}
	body := map[string]any{"name": name, "mimeType": folderMIME}
	if parent != "root" {
		body["parents"] = []string{parent}
	}
	cr, err := d.do(ctx, "POST", d.apiBase+"/drive/v3/files?fields=id", body, nil)
	if err != nil {
		return "", err
	}
	defer cr.Body.Close()
	if err := okStatus(cr); err != nil {
		return "", err
	}
	var made struct{ ID string }
	if err := json.NewDecoder(io.LimitReader(cr.Body, 1<<16)).Decode(&made); err != nil || made.ID == "" {
		return "", fmt.Errorf("notes: drive made no folder")
	}
	return made.ID, nil
}

// accessToken returns a cached access token, refreshing it a minute before it ends.
func (d *Drive) accessToken(ctx context.Context) (string, error) {
	d.tmu.Lock()
	defer d.tmu.Unlock()
	if d.tok != "" && d.now().Before(d.tokExp.Add(-time.Minute)) {
		return d.tok, nil
	}
	form := url.Values{
		"client_id": {d.cfg.ClientID}, "client_secret": {d.cfg.ClientSecret},
		"refresh_token": {d.cfg.RefreshToken}, "grant_type": {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", d.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := okStatus(resp); err != nil {
		return "", fmt.Errorf("notes: google sign-in failed: %w", err)
	}
	var t struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&t); err != nil || t.AccessToken == "" {
		return "", fmt.Errorf("notes: google sent no access token")
	}
	d.tok, d.tokExp = t.AccessToken, d.now().Add(time.Duration(t.ExpiresIn)*time.Second)
	return d.tok, nil
}

// do sends an authorised request; a non-nil body is sent as JSON.
func (d *Drive) do(ctx context.Context, method, u string, body any, hdr map[string]string) (*http.Response, error) {
	at, err := d.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+at)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return d.http.Do(req)
}

// okStatus turns a non-2xx reply into an error carrying a short, token-free snippet.
func okStatus(resp *http.Response) error {
	if resp.StatusCode/100 == 2 {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return fmt.Errorf("notes: google answered %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}
