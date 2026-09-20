//! Transaction endpoint tests: list (merged name), balance/totals, update, delete.

use axum::http::StatusCode;

use crate::tests::harness::TestApp;

#[tokio::test]
async fn create_list_returns_merged_name() {
    let app = TestApp::new().await;

    // Rule renames anything containing "zomato".
    let rule = app
        .post_json(
            "/api/rules",
            &serde_json::json!({
                "name": "Zomato", "priority": 5, "logic": "AND",
                "conditions": [{ "matchField": "NAME", "operator": "CONTAINS", "pattern": "zomato" }],
                "actions": [{ "setName": "Zomato Food", "setNameOp": "RENAME" }]
            }),
            Some(&app.token),
        )
        .await;
    assert_eq!(
        rule.0,
        StatusCode::CREATED,
        "rule create failed: {}",
        rule.1
    );

    assert_eq!(
        app.create_txn("ZOMATO-123", 100.0, "DEBIT").await.0,
        StatusCode::CREATED
    );
    assert_eq!(
        app.create_txn("Plain Shop", 5.0, "DEBIT").await.0,
        StatusCode::CREATED
    );

    let (status, list) = app
        .get_json("/api/transactions?pageSize=10", Some(&app.token))
        .await;
    assert_eq!(status, StatusCode::OK);
    let txns = list["transactions"].as_array().unwrap();
    assert_eq!(txns.len(), 2);

    // Write-time rule applied: wire returns the merged display name only.
    let names: Vec<&str> = txns.iter().map(|t| t["name"].as_str().unwrap()).collect();
    assert!(names.contains(&"Zomato Food"), "got {names:?}");
    assert!(names.contains(&"Plain Shop"), "got {names:?}");
    assert!(
        txns.iter().all(|t| t.get("cleanName").is_none()),
        "cleanName must not be on the wire"
    );
}

#[tokio::test]
async fn balance_and_totals_update() {
    let app = TestApp::new().await;

    app.create_txn("Salary", 1000.0, "CREDIT").await;
    app.create_txn("Rent", 400.0, "DEBIT").await;

    let (_, accounts) = app.get_json("/api/accounts", Some(&app.token)).await;
    let acc = &accounts["accounts"].as_array().unwrap()[0];
    assert_eq!(acc["balance"].as_f64(), Some(600.0), "balance: {acc}");
}

#[tokio::test]
async fn update_clears_rule_name() {
    let app = TestApp::new().await;

    app.post_json(
        "/api/rules",
        &serde_json::json!({
            "name": "Zomato", "priority": 5, "logic": "AND",
            "conditions": [{ "matchField": "NAME", "operator": "CONTAINS", "pattern": "zomato" }],
            "actions": [{ "setName": "Zomato Food", "setNameOp": "RENAME" }]
        }),
        Some(&app.token),
    )
    .await;
    app.create_txn("ZOMATO-123", 100.0, "DEBIT").await;

    let (_, list) = app
        .get_json("/api/transactions?pageSize=10", Some(&app.token))
        .await;
    let id = list["transactions"][0]["id"].as_str().unwrap().to_string();

    let upd = app
        .request(
            "PUT",
            "/api/transactions",
            Some(serde_json::json!({
                "transactions": [{ "id": id, "name": "My Dinner", "amount": 100.0, "type": "DEBIT", "accountId": app.account_id }]
            })),
            Some(&app.token),
        )
        .await;
    assert_eq!(upd.0, StatusCode::OK, "update failed: {}", upd.1);

    let (_, list) = app
        .get_json("/api/transactions?pageSize=10", Some(&app.token))
        .await;
    assert_eq!(list["transactions"][0]["name"].as_str(), Some("My Dinner"));
}

#[tokio::test]
async fn delete_restores_balance() {
    let app = TestApp::new().await;
    app.create_txn("Rent", 400.0, "DEBIT").await;

    let (_, list) = app
        .get_json("/api/transactions?pageSize=10", Some(&app.token))
        .await;
    let id = list["transactions"][0]["id"].as_str().unwrap().to_string();

    let del = app
        .request(
            "DELETE",
            "/api/transactions",
            Some(serde_json::json!({ "ids": [id] })),
            Some(&app.token),
        )
        .await;
    assert_eq!(del.0, StatusCode::NO_CONTENT);

    let (_, accounts) = app.get_json("/api/accounts", Some(&app.token)).await;
    assert_eq!(accounts["accounts"][0]["balance"].as_f64(), Some(0.0));
}
