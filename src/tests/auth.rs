//! Auth endpoint tests: signup, login, credential checks, auth gating.

use axum::http::StatusCode;

use crate::tests::harness::TestApp;

#[tokio::test]
async fn signup_login_and_reject_bad_credentials() {
    let app = TestApp::new().await;

    // Duplicate signup is rejected.
    let dup = app
        .post_json(
            "/api/auth/signup",
            &serde_json::json!({ "email": "test@x.com", "password": "secret1", "name": "Test" }),
            None,
        )
        .await;
    assert_eq!(dup.0, StatusCode::INTERNAL_SERVER_ERROR);

    // Good login returns tokens.
    let ok = app
        .post_json(
            "/api/auth/login",
            &serde_json::json!({ "email": "test@x.com", "password": "secret1" }),
            None,
        )
        .await;
    assert_eq!(ok.0, StatusCode::OK);
    assert!(ok.1["accessToken"].as_str().unwrap_or("").len() > 20);

    // Bad password is rejected.
    let bad = app
        .post_json(
            "/api/auth/login",
            &serde_json::json!({ "email": "test@x.com", "password": "wrong" }),
            None,
        )
        .await;
    assert_eq!(bad.0, StatusCode::UNAUTHORIZED);
}

#[tokio::test]
async fn protected_route_requires_auth() {
    let app = TestApp::new().await;
    let (status, _) = app.get_json("/api/accounts", None).await;
    assert_eq!(status, StatusCode::UNAUTHORIZED);
}
