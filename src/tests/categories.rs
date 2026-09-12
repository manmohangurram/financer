//! Category endpoint tests: list, create, transaction assignment.

use axum::http::StatusCode;

use crate::tests::harness::TestApp;

#[tokio::test]
async fn category_crud_and_transaction_assignment() {
    let app = TestApp::new().await;

    let created = app
        .post_json(
            "/api/categories",
            &serde_json::json!({ "categories": [{ "name": "Food" }] }),
            Some(&app.token),
        )
        .await;
    assert_eq!(created.0, StatusCode::CREATED, "cat create: {}", created.1);

    let (_, list) = app.get_json("/api/categories", Some(&app.token)).await;
    assert_eq!(list["categories"].as_array().map(Vec::len), Some(1));
    let cat_id = list["categories"][0]["id"].as_str().unwrap().to_string();

    // Create a txn assigned to the category; the list must return its id.
    app.post_json(
        "/api/transactions",
        &serde_json::json!({
            "transactions": [{ "name": "Lunch", "amount": 12.0, "type": "DEBIT",
                               "accountId": app.account_id, "categoryIds": [cat_id] }]
        }),
        Some(&app.token),
    )
    .await;

    let (_, txns) = app.get_json("/api/transactions?pageSize=10", Some(&app.token)).await;
    let cats = txns["transactions"][0]["categoryIds"].as_array().unwrap();
    assert_eq!(cats.len(), 1);
    assert_eq!(cats[0].as_str(), Some(cat_id.as_str()));
}
