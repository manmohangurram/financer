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
            &serde_json::json!({ "bankName": "Vanguard", "nickname": "Invest", "endingNumbers": "5678", "type": "SAVINGS" }),
            Some(&app.token),
        )
        .await;
    assert_eq!(created.0, StatusCode::CREATED);
    assert_eq!(created.1["endingNumbers"], "5678");
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
    // An omitted value on update keeps the stored one.
    assert_eq!(updated.1["endingNumbers"], "5678");

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

#[tokio::test]
async fn ending_numbers_is_required_and_four_digits() {
    let app = TestApp::new().await;

    for (value, why) in [
        (serde_json::json!(null), "missing"),
        (serde_json::json!(""), "empty"),
        (serde_json::json!("123"), "too short"),
        (serde_json::json!("12345"), "too long"),
        (serde_json::json!("12a4"), "non-digit"),
    ] {
        let mut body = serde_json::json!({ "bankName": "Chase", "type": "CURRENT" });
        if !value.is_null() {
            body["endingNumbers"] = value.clone();
        }
        let (status, _) = app
            .post_json("/api/accounts", &body, Some(&app.token))
            .await;
        assert_eq!(
            status,
            StatusCode::BAD_REQUEST,
            "create should reject {why}"
        );
    }

    let ok = app
        .post_json(
            "/api/accounts",
            &serde_json::json!({ "bankName": "Chase", "type": "CREDIT_CARD", "endingNumbers": "0007" }),
            Some(&app.token),
        )
        .await;
    assert_eq!(
        ok.0,
        StatusCode::CREATED,
        "card accounts use the same field"
    );
    let id = ok.1["id"].as_str().unwrap().to_string();

    // An update cannot clear it.
    let cleared = app
        .request(
            "PUT",
            &format!("/api/accounts/{id}"),
            Some(serde_json::json!({ "bankName": "Chase", "endingNumbers": "", "type": "CREDIT_CARD" })),
            Some(&app.token),
        )
        .await;
    assert_eq!(cleared.0, StatusCode::OK);
    assert_eq!(cleared.1["endingNumbers"], "0007");

    // But a bad value is rejected.
    let bad = app
        .request(
            "PUT",
            &format!("/api/accounts/{id}"),
            Some(serde_json::json!({ "bankName": "Chase", "endingNumbers": "abcd", "type": "CREDIT_CARD" })),
            Some(&app.token),
        )
        .await;
    assert_eq!(bad.0, StatusCode::BAD_REQUEST);
}
