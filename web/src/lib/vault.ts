import { revive, type Stored } from "./cache";

// Everything lives in this browser only. Clearing it (logout) is the kill switch.
//
// Credentials are encrypted with an AES-GCM key the browser generates as
// NON-EXTRACTABLE and keeps in IndexedDB: our own code can use it to decrypt for
// "sync", but can never read the key bytes out. This stops casual snooping of
// localStorage; it is not protection against a fully compromised device.

const DB = "meowance";
const STORE = "keys";
const KEY_ID = "creds";
const CREDS = "meowance.creds";
const DATA = "meowance.data";

export type Creds = { usn: string; dob: string };

function idb<T>(fn: (store: IDBObjectStore) => IDBRequest<T>, mode: IDBTransactionMode): Promise<T> {
  return new Promise((resolve, reject) => {
    const open = indexedDB.open(DB, 1);
    open.onupgradeneeded = () => open.result.createObjectStore(STORE);
    open.onerror = () => reject(open.error);
    open.onsuccess = () => {
      const tx = open.result.transaction(STORE, mode);
      const req = fn(tx.objectStore(STORE));
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
      tx.oncomplete = () => open.result.close();
    };
  });
}

async function key(): Promise<CryptoKey> {
  const existing = await idb<CryptoKey | undefined>((s) => s.get(KEY_ID), "readonly");
  if (existing) return existing;
  const k = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, false, ["encrypt", "decrypt"]);
  await idb((s) => s.put(k, KEY_ID), "readwrite");
  return k;
}

const b64 = (b: ArrayBuffer | Uint8Array) => btoa(String.fromCharCode(...new Uint8Array(b)));
const unb64 = (s: string) => Uint8Array.from(atob(s), (ch) => ch.charCodeAt(0));

export async function saveCreds(c: Creds): Promise<void> {
  try {
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const ct = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, await key(), new TextEncoder().encode(JSON.stringify(c)));
    localStorage.setItem(CREDS, JSON.stringify({ iv: b64(iv), ct: b64(ct) }));
  } catch {
    // storage blocked (private window etc.): sync will just ask them to log in again
  }
}

export async function loadCreds(): Promise<Creds | null> {
  try {
    const raw = localStorage.getItem(CREDS);
    if (!raw) return null;
    const { iv, ct } = JSON.parse(raw) as { iv: string; ct: string };
    const pt = await crypto.subtle.decrypt({ name: "AES-GCM", iv: unb64(iv) }, await key(), unb64(ct));
    return JSON.parse(new TextDecoder().decode(pt)) as Creds;
  } catch {
    return null;
  }
}

export function saveData(s: Stored): void {
  try {
    localStorage.setItem(DATA, JSON.stringify(s));
  } catch {
    /* ignore */
  }
}

export function loadData(): Stored | null {
  try {
    const raw = localStorage.getItem(DATA);
    return raw ? revive(JSON.parse(raw)) : null;
  } catch {
    return null;
  }
}

/** Logout: wipe creds, cached attendance and the key. */
export async function wipe(): Promise<void> {
  try {
    localStorage.removeItem(CREDS);
    localStorage.removeItem(DATA);
  } catch {
    /* ignore */
  }
  await new Promise<void>((resolve) => {
    const req = indexedDB.deleteDatabase(DB);
    req.onsuccess = req.onerror = req.onblocked = () => resolve();
  });
}
