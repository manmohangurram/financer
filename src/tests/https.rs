//! The HTTPS guard: credential-bearing endpoints refuse plaintext.

use axum::body::Body;
use axum::http::{Request, StatusCode};
use tower::ServiceExt;

use crate::tests::harness::TestApp;

async fn post_without_proto(app: &TestApp, path: &str) -> StatusCode {
    let req = Request::builder()
        .method("POST")
        .uri(path)
        .header("content-type", "application/json")
        .body(Body::from(r#"{"email":"a@b.c","password":"secret1"}"#))
        .unwrap();
    app.router.clone().oneshot(req).await.unwrap().status()
}

#[tokio::test]
async fn login_is_refused_over_plain_http() {
    let app = TestApp::new().await;
    assert_eq!(
        post_without_proto(&app, "/api/auth/login").await,
        StatusCode::FORBIDDEN
    );
}

#[tokio::test]
async fn mailbox_linking_is_refused_over_plain_http() {
    let app = TestApp::new().await;
    let req = Request::builder()
        .method("POST")
        .uri("/api/mail/accounts")
        .header("content-type", "application/json")
        .header("authorization", format!("Bearer {}", app.token))
        .body(Body::from("{}"))
        .unwrap();
    let res = app.router.clone().oneshot(req).await.unwrap();
    assert_eq!(res.status(), StatusCode::FORBIDDEN);
}

#[tokio::test]
async fn ordinary_reads_still_work_over_plain_http() {
    let app = TestApp::new().await;
    let req = Request::builder()
        .method("GET")
        .uri("/api/accounts")
        .header("authorization", format!("Bearer {}", app.token))
        .body(Body::empty())
        .unwrap();
    let res = app.router.clone().oneshot(req).await.unwrap();
    assert_eq!(
        res.status(),
        StatusCode::OK,
        "the guard is scoped to credentials"
    );
}
