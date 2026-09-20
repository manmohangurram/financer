//! MCP server — exposes financer data to external AI agents over the Model
//! Context Protocol. Mounted as a streamable-HTTP server at `/mcp`. Read tools
//! cover accounts/transactions/categories/investments; rule + category tools
//! allow a client (e.g. an AI agent) to manage rules and categories. Auth uses
//! the per-user API key from the `Authorization` header; the authenticated
//! `user_id` is injected into the request extensions and read by each tool
//! method (per-request, no shared mutable state).

/// Auth preamble for a tool: resolve the user id or return an error string.
/// `return` targets the calling tool fn. Usage: `let uid = uid!(&ctx);`
macro_rules! uid {
    ($ctx:expr) => {
        match McpHandler::user_id($ctx) {
            Ok(u) => u,
            Err(e) => return err_json(e.message),
        }
    };
}

pub mod handler;
pub mod models;
pub mod router;

pub use router::mcp_router;
