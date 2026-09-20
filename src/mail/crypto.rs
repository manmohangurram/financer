//! Credential encryption for linked mailboxes.
//!
//! The stored secret is the IMAP password, so it never sits in the clear. The
//! key is `FINANCER_SECRET_KEY` — deliberately **not** the JWT signing secret,
//! so rotating tokens cannot lock anyone out of their mailbox.

use base64::Engine;
use chacha20poly1305::aead::{Aead, AeadCore, KeyInit, OsRng};
use chacha20poly1305::{ChaCha20Poly1305, Nonce};

use crate::error::{ApiError, Result};

const NONCE_LEN: usize = 12;

pub fn decrypt(key_hex: &str, stored: &str) -> Result<String> {
    let blob = base64::engine::general_purpose::STANDARD
        .decode(stored)
        .map_err(|_| ApiError::internal("stored credential is not valid base64"))?;
    if blob.len() <= NONCE_LEN {
        return Err(ApiError::internal("stored credential is truncated"));
    }
    let (nonce, ciphertext) = blob.split_at(NONCE_LEN);
    let plaintext = cipher(key_hex)?
        .decrypt(Nonce::from_slice(nonce), ciphertext)
        .map_err(|_| {
            ApiError::bad_request(
                "could not decrypt the stored credential (wrong FINANCER_SECRET_KEY?)",
            )
        })?;
    String::from_utf8(plaintext).map_err(|_| ApiError::internal("stored credential is not UTF-8"))
}

fn cipher(key_hex: &str) -> Result<ChaCha20Poly1305> {
    let key = decode_key(key_hex)?;
    Ok(ChaCha20Poly1305::new((&key).into()))
}

/// 32 bytes of hex. Config validates this at boot, so a failure here means the
/// key was changed under a running process.
fn decode_key(key_hex: &str) -> Result<[u8; 32]> {
    if key_hex.len() != 64 {
        return Err(ApiError::bad_request(
            "FINANCER_SECRET_KEY must be 64 hex characters",
        ));
    }
    let mut key = [0u8; 32];
    for (i, byte) in key.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&key_hex[i * 2..i * 2 + 2], 16)
            .map_err(|_| ApiError::bad_request("FINANCER_SECRET_KEY is not valid hex"))?;
    }
    Ok(key)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// `nonce || ciphertext`, base64. A fresh random nonce per call.
    pub fn encrypt(key_hex: &str, plaintext: &str) -> Result<String> {
        let cipher = cipher(key_hex)?;
        let nonce = ChaCha20Poly1305::generate_nonce(&mut OsRng);
        let ciphertext = cipher
            .encrypt(&nonce, plaintext.as_bytes())
            .map_err(|_| ApiError::internal("credential encryption failed"))?;
        let mut blob = nonce.to_vec();
        blob.extend_from_slice(&ciphertext);
        Ok(base64::engine::general_purpose::STANDARD.encode(blob))
    }

    const KEY: &str = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";
    const OTHER: &str = "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100";

    #[test]
    fn round_trips_a_password() {
        let stored = encrypt(KEY, "hunter2").unwrap();
        assert_ne!(stored, "hunter2", "must not be stored in the clear");
        assert_eq!(decrypt(KEY, &stored).unwrap(), "hunter2");
    }

    #[test]
    fn a_fresh_nonce_every_time() {
        let a = encrypt(KEY, "same").unwrap();
        let b = encrypt(KEY, "same").unwrap();
        assert_ne!(a, b, "identical ciphertext would leak equality");
        assert_eq!(decrypt(KEY, &a).unwrap(), "same");
        assert_eq!(decrypt(KEY, &b).unwrap(), "same");
    }

    #[test]
    fn the_wrong_key_is_rejected_not_garbage() {
        let stored = encrypt(KEY, "hunter2").unwrap();
        assert!(decrypt(OTHER, &stored).is_err());
    }

    #[test]
    fn tampering_is_detected() {
        let stored = encrypt(KEY, "hunter2").unwrap();
        let mut blob = base64::engine::general_purpose::STANDARD
            .decode(&stored)
            .unwrap();
        let last = blob.len() - 1;
        blob[last] ^= 0x01;
        let tampered = base64::engine::general_purpose::STANDARD.encode(blob);
        assert!(
            decrypt(KEY, &tampered).is_err(),
            "AEAD must reject a flipped bit"
        );
    }

    #[test]
    fn rejects_bad_keys_and_truncated_blobs() {
        assert!(encrypt("abcd", "x").is_err());
        assert!(encrypt(&"z".repeat(64), "x").is_err());
        let short = base64::engine::general_purpose::STANDARD.encode([0u8; 4]);
        assert!(decrypt(KEY, &short).is_err());
    }
}
