//! Strangler gateway: forward requests for routes Rust doesn't own yet to the
//! Go backend. Mirror the response (status, headers, body) byte-for-byte.

use axum::body::Body;
use axum::http::{Request, Response, StatusCode, Uri};
use axum::response::IntoResponse;
use http_body_util::BodyExt;
use hyper::body::Incoming;
use hyper_util::client::legacy::connect::HttpConnector;
use hyper_util::client::legacy::Client;
use hyper_util::rt::TokioExecutor;

/// Reverse proxy a request to the Go backend at `target` origin (no trailing slash).
pub async fn proxy(req: Request<Body>, target: &str) -> Response<Body> {
    let client: Client<HttpConnector, Body> =
        Client::builder(TokioExecutor::new()).build(HttpConnector::new());

    let uri = build_uri(req.uri(), target);
    let (mut parts, body) = req.into_parts();
    parts.uri = uri;
    // axum sets Host to our own address; rewrite to target host.
    parts.headers.remove("host");
    let fwd = Request::from_parts(parts, body);

    match client.request(fwd).await {
        Ok(resp) => {
            let (parts, body) = resp.into_parts();
            let body = incoming_to_body(body).await;
            Response::from_parts(parts, body)
        }
        Err(e) => (
            StatusCode::BAD_GATEWAY,
            [(axum::http::header::CONTENT_TYPE, "application/json")],
            format!("{{\"code\":\"upstream_unavailable\",\"message\":\"{e}\"}}"),
        )
            .into_response(),
    }
}

async fn incoming_to_body(incoming: Incoming) -> Body {
    let data = incoming.collect().await.map(http_body_util::Collected::to_bytes).unwrap_or_default();
    Body::from(data)
}

fn build_uri(orig: &Uri, target: &str) -> Uri {
    let path = orig.path_and_query().map_or("/", axum::http::uri::PathAndQuery::as_str);
    format!("{target}{path}")
        .parse()
        .unwrap_or_else(|_| "/".parse().unwrap())
}
