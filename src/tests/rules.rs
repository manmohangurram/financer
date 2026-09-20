//! Rule endpoint tests: create, list, preview, delete.

use axum::http::StatusCode;

use crate::tests::harness::TestApp;

#[tokio::test]
async fn rule_crud_and_preview() {
    let app = TestApp::new().await;

    let created = app
        .post_json(
            "/api/rules",
            &serde_json::json!({
                "name": "Coffee", "priority": 3, "logic": "AND",
                "conditions": [{ "matchField": "NAME", "operator": "CONTAINS", "pattern": "coffee" }],
                "actions": [{ "setName": "Coffee Shop", "setNameOp": "RENAME" }]
            }),
            Some(&app.token),
        )
        .await;
    assert_eq!(created.0, StatusCode::CREATED, "rule create: {}", created.1);

    let (_, rules) = app.get_json("/api/rules", Some(&app.token)).await;
    assert_eq!(rules["rules"].as_array().map(Vec::len), Some(1));

    // Preview matches an existing txn against the rule's conditions.
    let preview = app
        .post_json(
            "/api/rules/preview",
            &serde_json::json!({
                "logic": "AND",
                "conditions": [{ "matchField": "NAME", "operator": "CONTAINS", "pattern": "coffee" }]
            }),
            Some(&app.token),
        )
        .await;
    assert_eq!(preview.0, StatusCode::OK, "preview: {}", preview.1);
    assert!(
        preview.1.get("transactions").is_some() || preview.1.is_array(),
        "preview body: {}",
        preview.1
    );

    // Delete the rule.
    let rule_id = rules["rules"][0]["id"].as_str().unwrap().to_string();
    let del = app
        .request(
            "DELETE",
            &format!("/api/rules/{rule_id}"),
            None,
            Some(&app.token),
        )
        .await;
    assert_eq!(del.0, StatusCode::NO_CONTENT);
    let (_, rules) = app.get_json("/api/rules", Some(&app.token)).await;
    assert_eq!(rules["rules"].as_array().map(Vec::len), Some(0));
}
