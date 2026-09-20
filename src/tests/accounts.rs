//! Account endpoint tests: list, create, update, delete.

use axum::http::StatusCode;

use crate::tests::harness::TestApp;

#[tokio::test]
async fn account_crud_roundtrip() {
    let app = TestApp::new().await;

    let (status, list) = app.get_json("/api/accounts", Some(&app.token)).await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(list["accounts"].as_array().map(Vec::len), Some(1));

    // Create a second account.
    let created = app
        .post_json(
            "/api/accounts",
            &serde_json::json!({ "bankName": "Vanguard", "nickname": "Invest", "type": "SAVINGS" }),
            Some(&app.token),
        )
        .await;
    assert_eq!(created.0, StatusCode::CREATED);
    let id = created.1["id"].as_str().unwrap().to_string();

    // Update it.
    let updated = app
        .request(
            "PUT",
            &format!("/api/accounts/{id}"),
            Some(serde_json::json!({ "bankName": "Vanguard", "nickname": "Brokerage", "type": "SAVINGS" })),
            Some(&app.token),
        )
        .await;
    assert_eq!(updated.0, StatusCode::OK);

    // Delete it.
    let deleted = app
        .request(
            "DELETE",
            &format!("/api/accounts/{id}"),
            None,
            Some(&app.token),
        )
        .await;
    assert_eq!(deleted.0, StatusCode::NO_CONTENT);

    let (_, list) = app.get_json("/api/accounts", Some(&app.token)).await;
    assert_eq!(list["accounts"].as_array().map(Vec::len), Some(1));
}
