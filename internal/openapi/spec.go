// Package openapi serves the hand-written OpenAPI description and a Swagger UI
// page at /docs. It covers the routes this backend implements; the removed
// statement-import routes are intentionally absent.
package openapi

import "strings"

// Spec is the OpenAPI 3.0 document for the Financer API.
func Spec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Financer API",
			"description": "Personal finance tracker — REST JSON API. All endpoints except /api/auth/* require `Authorization: Bearer <accessToken>`.",
			"version":     "0.4.0",
		},
		"security": []any{map[string]any{"bearer_auth": []any{}}},
		"tags": []any{
			tag("auth", "Authentication"),
			tag("accounts", "Bank accounts"),
			tag("transactions", "Transactions & transfers"),
			tag("categories", "Categories"),
			tag("rules", "Auto-categorization rules"),
			tag("analytics", "Dashboard & spending"),
			tag("settings", "Profile, password, avatar, API keys"),
			tag("mail", "Mailbox ingest"),
		},
		"paths":      paths(),
		"components": components(),
	}
}

func tag(name, desc string) map[string]any {
	return map[string]any{"name": name, "description": desc}
}

// paginated is a list operation that accepts pageSize/pageToken.
func paginated(tagName, summary, schema string) map[string]any {
	o := op(tagName, summary, "", okResp(summary, schema))
	o["parameters"] = []any{
		map[string]any{"name": "pageSize", "in": "query", "schema": map[string]any{"type": "integer"}},
		map[string]any{"name": "pageToken", "in": "query", "schema": map[string]any{"type": "string"}},
	}
	return o
}

func op(tagName, summary string, body string, ok map[string]any) map[string]any {
	o := map[string]any{
		"tags":    []any{tagName},
		"summary": summary,
		"responses": map[string]any{
			"200": ok,
			"400": errRef("invalid_argument"),
			"401": errRef("unauthenticated"),
		},
	}
	if body != "" {
		o["requestBody"] = map[string]any{
			"required": true,
			"content":  map[string]any{"application/json": map[string]any{"schema": ref(body)}},
		}
	}
	return o
}

func okResp(desc, schema string) map[string]any {
	content := map[string]any{}
	if schema != "" {
		content["application/json"] = map[string]any{"schema": ref(schema)}
	}
	return map[string]any{"description": desc, "content": content}
}

func noContent(desc string) map[string]any {
	return map[string]any{"description": desc}
}

func errRef(name string) map[string]any {
	return map[string]any{
		"description": name,
		"content":     map[string]any{"application/json": map[string]any{"schema": ref("Error")}},
	}
}

func ref(schema string) map[string]any {
	if strings.HasPrefix(schema, "[]") {
		return map[string]any{"type": "array", "items": ref(strings.TrimPrefix(schema, "[]"))}
	}
	return map[string]any{"$ref": "#/components/schemas/" + schema}
}

func idParam() map[string]any {
	return map[string]any{
		"name": "id", "in": "path", "required": true,
		"schema": map[string]any{"type": "string"},
	}
}

func paths() map[string]any {
	return map[string]any{
		"/api/auth/signup": map[string]any{
			"post": op("auth", "Create an account and receive tokens", "SignupRequest", okResp("Tokens", "AuthResponse")),
		},
		"/api/auth/login": map[string]any{
			"post": op("auth", "Log in and receive tokens", "LoginRequest", okResp("Tokens", "AuthResponse")),
		},
		"/api/auth/refresh": map[string]any{
			"post": op("auth", "Rotate an access token", "RefreshRequest", okResp("Tokens", "AuthResponse")),
		},
		"/api/me/profile": map[string]any{
			"get": op("settings", "Read the profile", "", okResp("Profile", "ProfileResponse")),
			"put": op("settings", "Update the profile", "ReqProfile", okResp("Profile", "ProfileResponse")),
		},
		"/api/me/password": map[string]any{
			"post": op("settings", "Change the password", "ReqPassword", okResp("Tokens", "AuthResponse")),
		},
		"/api/me/logout-all": map[string]any{
			"post": op("settings", "Revoke all sessions", "", okResp("Result", "BulkResult")),
		},
		"/api/me/avatar": map[string]any{
			"post": map[string]any{
				"tags": []any{"settings"}, "summary": "Upload an avatar (multipart 'file')",
				"responses": map[string]any{"201": okResp("Profile", "ProfileResponse"), "400": errRef("invalid_argument"), "401": errRef("unauthenticated")},
			},
		},
		"/api/me/keys": map[string]any{
			"get":  op("settings", "List API keys (masked)", "", okResp("Keys", "[]UserKey")),
			"post": op("settings", "Create an API key (plaintext shown once)", "ReqCreateKey", okResp("Key", "CreatedUserKey")),
		},
		"/api/me/keys/{id}": map[string]any{
			"delete": map[string]any{
				"tags": []any{"settings"}, "summary": "Revoke an API key",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"204": noContent("Revoked"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/accounts": map[string]any{
			"get":  paginated("accounts", "List accounts", "ListAccounts"),
			"post": op("accounts", "Create an account", "ReqAccount", okResp("Account", "Account")),
		},
		"/api/accounts/{id}": map[string]any{
			"put": map[string]any{
				"tags": []any{"accounts"}, "summary": "Update an account",
				"parameters":  []any{idParam()},
				"requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": ref("ReqAccount")}}},
				"responses":   map[string]any{"200": okResp("Account", "Account"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
			"delete": map[string]any{
				"tags": []any{"accounts"}, "summary": "Delete an account",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"204": noContent("Deleted"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/categories": map[string]any{
			"get":    op("categories", "List categories (cursor paginated)", "", okResp("Categories", "ListCategories")),
			"post":   op("categories", "Create categories", "ReqCategoriesCreate", okResp("Result", "BulkResult")),
			"put":    op("categories", "Update categories", "ReqCategoriesUpdate", okResp("Result", "BulkResult")),
			"delete": op("categories", "Delete categories", "ReqCategoriesDelete", okResp("Result", "BulkResult")),
		},
		"/api/categories/{id}/budget": map[string]any{
			"put": map[string]any{
				"tags": []any{"categories"}, "summary": "Set or clear a category's recurring monthly budget (0 clears it)",
				"parameters": []any{idParam()},
				"requestBody": map[string]any{"required": true, "content": map[string]any{
					"application/json": map[string]any{"schema": ref("ReqCategoryBudget")}}},
				"responses": map[string]any{"200": okResp("Category", "Category"), "400": errRef("invalid_argument"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/transactions": map[string]any{
			"get":    op("transactions", "List transactions", "", okResp("Transactions", "ListTransactions")),
			"post":   op("transactions", "Create transactions", "ReqTransactions", okResp("Result", "BulkResult")),
			"put":    op("transactions", "Update transactions", "ReqTransactions", okResp("Result", "BulkResult")),
			"delete": op("transactions", "Delete transactions", "ReqTransactionIDs", okResp("Result", "BulkResult")),
		},
		"/api/transfer-links": map[string]any{
			"post":   op("transactions", "Link transfer debit/credit pairs", "ReqTransfer", okResp("Result", "BulkResult")),
			"delete": op("transactions", "Unlink transfers", "ReqTransfer", okResp("Result", "BulkResult")),
		},
		"/api/transfer-links/counterpart": map[string]any{
			"post": op("transactions", "Create the missing side of a transfer", "ReqCounterpart", okResp("Counterpart", "TransferCounterpart")),
		},
		"/api/rules": map[string]any{
			"get":  paginated("rules", "List rules", "ListRules"),
			"post": op("rules", "Create a rule", "ReqRule", okResp("Rule", "Rule")),
		},
		"/api/rules/{id}": map[string]any{
			"put": map[string]any{
				"tags": []any{"rules"}, "summary": "Update a rule",
				"parameters":  []any{idParam()},
				"requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": ref("ReqRule")}}},
				"responses":   map[string]any{"200": okResp("Rule", "Rule"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
			"delete": map[string]any{
				"tags": []any{"rules"}, "summary": "Delete a rule",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"204": noContent("Deleted"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/rules/preview": map[string]any{
			"post": op("rules", "Preview matching transactions", "ReqPreview", okResp("Matches", "ListPreview")),
		},
		"/api/rules/{id}/run": map[string]any{
			"post": map[string]any{
				"tags": []any{"rules"}, "summary": "Apply a rule to existing transactions",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"200": okResp("Run summary", "RuleRun"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/dashboard": map[string]any{
			"get": op("analytics", "Dashboard totals and accounts", "", okResp("Dashboard", "Dashboard")),
		},
		"/api/spending": map[string]any{
			"get": map[string]any{
				"tags": []any{"analytics"}, "summary": "Spending buckets and category totals",
				"parameters": []any{
					map[string]any{"name": "range", "in": "query", "schema": map[string]any{"type": "string", "enum": []any{"7D", "1M", "6M", "1Y"}}},
					map[string]any{"name": "from", "in": "query", "schema": map[string]any{"type": "string", "format": "date"}},
					map[string]any{"name": "to", "in": "query", "schema": map[string]any{"type": "string", "format": "date"}},
					map[string]any{"name": "accountId", "in": "query", "schema": map[string]any{"type": "string"}},
				},
				"responses": map[string]any{"200": okResp("Spending", "Spending"), "401": errRef("unauthenticated")},
			},
		},
		"/api/mail/accounts": map[string]any{
			"get":  paginated("mail", "List linked mailboxes", "ListMailAccounts"),
			"post": op("mail", "Link a mailbox", "ReqMailAccount", okResp("Mailbox", "MailAccount")),
		},
		"/api/mail/accounts/{id}": map[string]any{
			"put": map[string]any{
				"tags": []any{"mail"}, "summary": "Edit a mailbox (empty password keeps the stored one)",
				"parameters": []any{idParam()},
				"requestBody": map[string]any{"required": true, "content": map[string]any{
					"application/json": map[string]any{"schema": ref("ReqUpdateMailAccount")}}},
				"responses": map[string]any{"200": okResp("Mailbox", "MailAccount"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
			"delete": map[string]any{
				"tags": []any{"mail"}, "summary": "Remove a mailbox",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"204": noContent("Removed"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/mail/poll": map[string]any{
			"post": map[string]any{
				"tags": []any{"mail"}, "summary": "Start an asynchronous poll for the user's mailboxes",
				"parameters": []any{
					map[string]any{"name": "force", "in": "query", "schema": map[string]any{"type": "string", "enum": []any{"1", "true"}}},
				},
				"responses": map[string]any{"202": okResp("Poll status", "PollStatus"), "401": errRef("unauthenticated")},
			},
		},
		"/api/me/llm": map[string]any{
			"get": op("settings", "Read per-user Inbox AI settings (API key never returned)", "", okResp("Settings", "LLMSettings")),
			"put": map[string]any{
				"tags": []any{"settings"}, "summary": "Update per-user Inbox AI settings",
				"requestBody": map[string]any{"required": true, "content": map[string]any{
					"application/json": map[string]any{"schema": ref("ReqLLMSettings")}}},
				"responses": map[string]any{"200": okResp("Settings", "LLMSettings"), "400": errRef("invalid_argument"), "401": errRef("unauthenticated")},
			},
		},
		"/api/me/llm/test": map[string]any{
			"post": op("settings", "Test the configured LiteLLM endpoint", "", okResp("Result", "LLMTestResult")),
		},
		"/api/mail/accounts/{id}/poll": map[string]any{
			"post": map[string]any{
				"tags": []any{"mail"}, "summary": "Poll one mailbox now",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"200": okResp("Poll summary", "PollSummary"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/mail/poll/full": map[string]any{
			"post": map[string]any{
				"tags": []any{"mail"}, "summary": "Start a background poll of the whole mailbox",
				"responses": map[string]any{"202": okResp("Poll status", "PollStatus"), "401": errRef("unauthenticated")},
			},
		},
		"/api/mail/ai-parse": map[string]any{
			"get": op("mail", "Read the AI re-parse job state", "", okResp("Status", "AIParseStatus")),
			"post": map[string]any{
				"tags": []any{"mail"}, "summary": "Re-parse pending mail per sender with the model",
				"responses": map[string]any{"202": okResp("Status", "AIParseStatus"), "409": errRef("conflict"), "401": errRef("unauthenticated")},
			},
		},
		"/api/mail/messages": map[string]any{
			"get": map[string]any{
				"tags": []any{"mail"}, "summary": "List stored mail",
				"parameters": []any{
					map[string]any{"name": "status", "in": "query", "schema": map[string]any{"type": "string", "enum": []any{"pending", "unparsed", "ignored", "auto", "rejected", "review", "all"}}},
					map[string]any{"name": "pageSize", "in": "query", "schema": map[string]any{"type": "integer", "maximum": 100}},
					map[string]any{"name": "limit", "in": "query", "deprecated": true, "schema": map[string]any{"type": "integer", "maximum": 100}},
					map[string]any{"name": "accountId", "in": "query", "schema": map[string]any{"type": "string"}},
					map[string]any{"name": "search", "in": "query", "schema": map[string]any{"type": "string"}},
					map[string]any{"name": "pageToken", "in": "query", "schema": map[string]any{"type": "string"}},
				},
				"responses": map[string]any{"200": okResp("Messages", "ListMailMessages"), "400": errRef("invalid_argument"), "401": errRef("unauthenticated")},
			},
			"post": op("mail", "Approve or reject messages", "ReqMailAction", okResp("Outcome", "MailActionOutcome")),
		},
		"/api/mail/messages/{id}": map[string]any{
			"get": map[string]any{
				"tags": []any{"mail"}, "summary": "Read one message with its parsed body and raw source",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"200": okResp("Message", "MailMessage"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/mail/rules": map[string]any{
			"get":  paginated("mail", "List mail rules", "ListMailRules"),
			"post": op("mail", "Create a mail rule", "ReqMailRule", okResp("Rule", "MailRule")),
		},
		"/api/mail/rules/{id}": map[string]any{
			"put": map[string]any{
				"tags": []any{"mail"}, "summary": "Update a mail rule",
				"parameters":  []any{idParam()},
				"requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": ref("ReqMailRule")}}},
				"responses":   map[string]any{"204": noContent("Updated"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
			"delete": map[string]any{
				"tags": []any{"mail"}, "summary": "Delete a mail rule",
				"parameters": []any{idParam()},
				"responses":  map[string]any{"204": noContent("Deleted"), "404": errRef("not_found"), "401": errRef("unauthenticated")},
			},
		},
		"/api/mail/templates/validate": map[string]any{
			"post": op("mail", "Test manual template patterns against a sample mail", "ReqMailTemplateValidate", okResp("Validation", "MailTemplateValidation")),
		},
	}
}

func components() map[string]any {
	s := map[string]any{
		"Error": obj(map[string]any{
			"code":    map[string]any{"type": "string"},
			"message": map[string]any{"type": "string"},
		}),
		"AccountType":     map[string]any{"type": "string", "enum": []any{"CURRENT", "SAVINGS", "LOAN", "CREDIT_CARD"}},
		"TransactionType": map[string]any{"type": "string", "enum": []any{"DEBIT", "CREDIT"}},
		"Account": obj(map[string]any{
			"id": str(), "bankName": str(), "nickname": str(),
			"balance": map[string]any{"type": "number"},
			"type":    ref("AccountType"), "endingNumbers": nullable(str()),
			"createdAt": map[string]any{"type": "string", "format": "date-time"},
		}),
		"ReqAccount": obj(map[string]any{
			"id": str(), "bankName": str(), "nickname": str(),
			"endingNumbers": str(), "type": ref("AccountType"),
		}),
		"ListAccounts": obj(map[string]any{"accounts": ref("[]Account"), "nextPageToken": str()}),
		"Transaction": obj(map[string]any{
			"id": str(), "name": str(), "amount": map[string]any{"type": "number"},
			"type": ref("TransactionType"), "occurredAt": str(), "accountId": str(),
			"createdAt": str(), "linkedTransferId": str(),
			"categoryIds": nullable(map[string]any{"type": "array", "items": str()}),
		}),
		"ListTransactions": obj(map[string]any{
			"transactions":  ref("[]Transaction"),
			"nextPageToken": str(), "totalCount": map[string]any{"type": "integer"},
		}),
		"BulkResult": obj(map[string]any{
			"success": map[string]any{"type": "boolean"}, "message": str(),
			"failedIds": map[string]any{"type": "array", "items": str()},
			"skipped":   map[string]any{"type": "integer"},
		}),
		"Category": obj(map[string]any{
			"id": str(), "name": str(), "monthlyBudget": map[string]any{"type": "number"}, "createdAt": str(),
		}),
		"ReqCategoryBudget": obj(map[string]any{"monthlyBudget": map[string]any{"type": "number"}}),
		"ListCategories": obj(map[string]any{
			"categories": ref("[]Category"), "nextPageToken": str(),
		}),
		"RuleCondition": obj(map[string]any{
			"matchField": map[string]any{"type": "string", "enum": []any{"NAME", "AMOUNT", "TYPE", "CATEGORY", "ACCOUNT"}},
			"operator":   map[string]any{"type": "string", "enum": []any{"CONTAINS", "STARTS_WITH", "ENDS_WITH", "EQUALS", "GREATER_THAN", "LESS_THAN", "REGEX"}},
			"pattern":    str(),
		}),
		"RuleAction": obj(map[string]any{
			"setName":       str(),
			"setNameOp":     map[string]any{"type": "string", "enum": []any{"RENAME", "ADD_PREFIX", "ADD_SUFFIX"}},
			"setCategoryId": str(), "setTransferAccountId": str(),
		}),
		"Rule": obj(map[string]any{
			"id": str(), "name": str(), "priority": map[string]any{"type": "integer"},
			"logic":      map[string]any{"type": "string", "enum": []any{"OR", "AND"}},
			"conditions": ref("[]RuleCondition"), "actions": ref("[]RuleAction"),
			"createdAt": str(),
		}),
		"ListRules": obj(map[string]any{"rules": ref("[]Rule"), "nextPageToken": str()}),
		"ReqRule": obj(map[string]any{
			"name": str(), "priority": map[string]any{"type": "integer"},
			"logic":      map[string]any{"type": "string", "enum": []any{"OR", "AND"}},
			"conditions": ref("[]RuleCondition"), "actions": ref("[]RuleAction"),
		}),
		"ReqPreview": obj(map[string]any{
			"logic":      map[string]any{"type": "string", "enum": []any{"OR", "AND"}},
			"conditions": ref("[]RuleCondition"), "limit": map[string]any{"type": "integer"},
		}),
		"PreviewTxn": obj(map[string]any{
			"name": str(), "amount": map[string]any{"type": "number"}, "type": str(),
			"accountId": str(), "categoryIds": ref("[]Category"), "occurredAt": str(),
		}),
		"ListPreview": obj(map[string]any{"transactions": map[string]any{"type": "array", "items": ref("PreviewTxn")}}),
		"RuleRun": obj(map[string]any{
			"updated": map[string]any{"type": "integer"}, "matched": map[string]any{"type": "integer"},
			"linked": map[string]any{"type": "integer"}, "created": map[string]any{"type": "integer"},
		}),
		"Dashboard": obj(map[string]any{
			"totalBalance":  map[string]any{"type": "number"},
			"totalIncome":   map[string]any{"type": "number"},
			"totalExpenses": map[string]any{"type": "number"},
			"accounts":      ref("[]Account"),
		}),
		"SpendingBucket": obj(map[string]any{
			"key": str(), "label": str(), "amount": map[string]any{"type": "number"},
		}),
		"SpendingCategory": obj(map[string]any{
			"id": str(), "name": str(),
			"debit": map[string]any{"type": "number"}, "credit": map[string]any{"type": "number"},
			"net": map[string]any{"type": "number"},
		}),
		"Spending": obj(map[string]any{
			"buckets":    map[string]any{"type": "array", "items": ref("SpendingBucket")},
			"categories": map[string]any{"type": "array", "items": ref("SpendingCategory")},
		}),
		"AuthResponse": obj(map[string]any{
			"accessToken": str(), "refreshToken": str(), "userId": str(),
			"email": str(), "name": str(),
		}),
		"SignupRequest":  obj(map[string]any{"email": str(), "password": str(), "name": str()}),
		"LoginRequest":   obj(map[string]any{"email": str(), "password": str()}),
		"RefreshRequest": obj(map[string]any{"refreshToken": str()}),
		"ProfileResponse": obj(map[string]any{
			"userId": str(), "name": str(), "email": str(), "avatarUrl": str(),
		}),
		"ReqProfile":  obj(map[string]any{"name": str(), "email": str(), "avatarUrl": str()}),
		"ReqPassword": obj(map[string]any{"currentPassword": str(), "newPassword": str()}),
		"UserKey": obj(map[string]any{
			"id": str(), "name": str(), "keyPrefix": str(), "createdAt": str(),
			"expiresAt": nullable(str()), "lastUsedAt": nullable(str()),
			"scope": map[string]any{"type": "string", "enum": []any{"read", "read_write"}},
		}),
		"CreatedUserKey": obj(map[string]any{
			"id": str(), "name": str(), "key": str(), "keyPrefix": str(),
			"createdAt": str(), "expiresAt": nullable(str()),
			"lastUsedAt": nullable(str()),
			"scope":      map[string]any{"type": "string", "enum": []any{"read", "read_write"}},
		}),
		"ReqCreateKey": obj(map[string]any{
			"name": str(), "expiresInDays": map[string]any{"type": "integer"},
			"scope": map[string]any{"type": "string", "enum": []any{"read", "read_write"}},
		}),
		"ReqTransactions": obj(map[string]any{
			"transactions": map[string]any{"type": "array", "items": ref("TransactionInput")},
		}),
		"ReqTransactionIDs": obj(map[string]any{"ids": map[string]any{"type": "array", "items": str()}}),
		"TransactionInput": obj(map[string]any{
			"id": str(), "name": str(), "amount": map[string]any{"type": "number"},
			"type": ref("TransactionType"), "occurredAt": map[string]any{},
			"accountId": str(), "categoryIds": ref("[]Category"), "externalId": str(),
		}),
		"ReqCategoriesCreate": obj(map[string]any{
			"categories": map[string]any{"type": "array", "items": obj(map[string]any{
				"name": str(), "monthlyBudget": map[string]any{"type": "number"},
			})},
		}),
		"ReqCategoriesUpdate": obj(map[string]any{
			"categories": map[string]any{"type": "array", "items": obj(map[string]any{
				"id": str(), "name": str(), "monthlyBudget": map[string]any{"type": "number", "nullable": true},
			})},
		}),
		"ReqCategoriesDelete": obj(map[string]any{"ids": map[string]any{"type": "array", "items": str()}}),
		"ReqTransfer": obj(map[string]any{
			"ids": map[string]any{"type": "array", "items": str()},
			"links": map[string]any{"type": "array", "items": obj(map[string]any{
				"debitTransactionId": str(), "creditTransactionId": str(),
			})},
		}),
		"ReqCounterpart":      obj(map[string]any{"transactionId": str(), "toAccountId": str()}),
		"TransferCounterpart": obj(map[string]any{"debitTransactionId": str(), "creditTransactionId": str()}),
		"MailAccount": obj(map[string]any{
			"id": str(), "label": str(), "imapHost": str(),
			"imapPort": map[string]any{"type": "integer"}, "username": str(),
			"enabled":      map[string]any{"type": "boolean"},
			"lastPolledAt": nullable(str()), "lastError": str(),
		}),
		"ReqMailAccount": obj(map[string]any{
			"label": str(), "imapHost": str(), "imapPort": map[string]any{"type": "integer"},
			"username": str(), "password": str(),
		}),
		"ReqUpdateMailAccount": obj(map[string]any{
			"label": str(), "imapHost": str(), "imapPort": map[string]any{"type": "integer"},
			"username": str(), "password": str(), "enabled": map[string]any{"type": "boolean"},
		}),
		"PollStatus": obj(map[string]any{
			"status": map[string]any{"type": "string", "enum": []any{"started", "throttled"}},
		}),
		"AIParseStatus": obj(map[string]any{
			"running":       map[string]any{"type": "boolean"},
			"totalSenders":  map[string]any{"type": "integer"},
			"doneSenders":   map[string]any{"type": "integer"},
			"currentSender": str(),
			"startedAt":     str(),
			"finishedAt":    str(),
			"error":         str(),
		}),
		"LLMSettings": obj(map[string]any{
			"baseUrl": str(), "model": str(),
			"timeoutSecs":             map[string]any{"type": "integer"},
			"enabled":                 map[string]any{"type": "boolean"},
			"apiKeySet":               map[string]any{"type": "boolean"},
			"minSamples":              map[string]any{"type": "integer"},
			"failureThreshold":        map[string]any{"type": "integer"},
			"cooldownMinutes":         map[string]any{"type": "integer"},
			"autoPollThrottleMinutes": map[string]any{"type": "integer"},
			"reasoningEnabled":        map[string]any{"type": "boolean"},
			"reasoningEffort":         str(),
			"updatedAt":               str(),
		}),
		"ReqLLMSettings": obj(map[string]any{
			"baseUrl": str(), "model": str(),
			"apiKey":                  map[string]any{"type": "string", "nullable": true},
			"timeoutSecs":             map[string]any{"type": "integer"},
			"enabled":                 map[string]any{"type": "boolean"},
			"minSamples":              map[string]any{"type": "integer"},
			"failureThreshold":        map[string]any{"type": "integer"},
			"cooldownMinutes":         map[string]any{"type": "integer"},
			"autoPollThrottleMinutes": map[string]any{"type": "integer"},
			"reasoningEnabled":        map[string]any{"type": "boolean"},
			"reasoningEffort":         map[string]any{"type": "string", "enum": []any{"", "low", "medium", "high"}},
		}),
		"LLMTestResult": obj(map[string]any{
			"ok": map[string]any{"type": "boolean"}, "message": str(),
		}),
		"ListMailAccounts": obj(map[string]any{"accounts": ref("[]MailAccount"), "nextPageToken": str()}),
		"MailMessage": obj(map[string]any{
			"id": str(), "messageId": str(), "mailAccountId": str(),
			"fromAddr": str(), "fromName": str(), "subject": str(),
			"receivedAt": nullable(str()), "status": str(), "parsedJson": nullable(str()),
			"confidence":    map[string]any{"type": "number"},
			"bodyPreview":   str(),
			"bodyText":      nullable(str()),
			"rawText":       nullable(str()),
			"transactionId": nullable(str()),
		}),
		"ListMailMessages": obj(map[string]any{
			"messages":      ref("[]MailMessage"),
			"total":         map[string]any{"type": "integer"},
			"totalCount":    map[string]any{"type": "integer"},
			"nextPageToken": str(),
		}),
		"MailRule": obj(map[string]any{
			"id": str(), "position": map[string]any{"type": "integer"},
			"enabled": map[string]any{"type": "boolean"}, "fromPattern": str(),
			"subjectContains": str(), "bodyContains": str(),
			"action":      map[string]any{"type": "string", "enum": []any{"IGNORE", "REVIEW", "AUTO"}},
			"accountId":   nullable(str()),
			"categoryIds": map[string]any{"type": "array", "items": str()},
			"template":    ref("MailTemplatePatterns"),
		}),
		"ReqMailRule": obj(map[string]any{
			"position": map[string]any{"type": "integer"}, "enabled": map[string]any{"type": "boolean"},
			"fromPattern": str(), "subjectContains": str(), "bodyContains": str(),
			"action":      map[string]any{"type": "string", "enum": []any{"IGNORE", "REVIEW", "AUTO"}},
			"accountId":   nullable(str()),
			"categoryIds": map[string]any{"type": "array", "items": str()},
			"template":    ref("MailTemplatePatterns"),
		}),
		"MailTemplatePatterns": obj(map[string]any{
			"amount":   map[string]any{"type": "array", "items": str()},
			"debit":    map[string]any{"type": "array", "items": str()},
			"credit":   map[string]any{"type": "array", "items": str()},
			"account":  map[string]any{"type": "array", "items": str()},
			"merchant": map[string]any{"type": "array", "items": str()},
			"date":     map[string]any{"type": "array", "items": str()},
		}),
		"ReqMailTemplateValidate": obj(map[string]any{
			"subject": str(), "body": str(), "patterns": ref("MailTemplatePatterns"),
		}),
		"MailTemplateValidation": obj(map[string]any{
			"fields": map[string]any{"type": "array", "items": obj(map[string]any{
				"field": str(),
				"patterns": map[string]any{"type": "array", "items": obj(map[string]any{
					"pattern": str(), "matched": map[string]any{"type": "boolean"},
					"value": str(), "error": str(),
				})},
			})},
			"amount":    map[string]any{"type": "number"},
			"direction": str(), "account": str(), "merchant": str(), "date": str(),
			"complete": map[string]any{"type": "boolean"},
			"reason":   str(),
		}),
		"ListMailRules": obj(map[string]any{"rules": ref("[]MailRule"), "nextPageToken": str()}),
		"ReqMailAction": obj(map[string]any{
			"messages": map[string]any{"type": "array", "items": obj(map[string]any{
				"id": str(), "action": map[string]any{"type": "string", "enum": []any{"APPROVE", "REJECT"}},
				"accountId":   nullable(str()),
				"categoryIds": map[string]any{"type": "array", "items": str()},
				"name":        nullable(str()),
				"amount":      map[string]any{"type": "number", "nullable": true},
				"type":        nullable(map[string]any{"type": "string", "enum": []any{"DEBIT", "CREDIT"}}),
				"occurredAt":  nullable(str()),
			})},
		}),
		"MailActionOutcome": obj(map[string]any{
			"succeeded": map[string]any{"type": "array", "items": str()},
			"failed":    map[string]any{"type": "array", "items": obj(map[string]any{"id": str(), "error": str()})},
		}),
		"PollSummary": obj(map[string]any{
			"matched": map[string]any{"type": "integer"},
			"stored":  map[string]any{"type": "integer"},
			"skipped": map[string]any{"type": "integer"},
		}),
	}
	return map[string]any{
		"schemas":         s,
		"securitySchemes": map[string]any{"bearer_auth": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}},
	}
}

func obj(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}

func str() map[string]any { return map[string]any{"type": "string"} }

func nullable(schema map[string]any) map[string]any {
	if _, ok := schema["$ref"]; ok {
		return map[string]any{"allOf": []any{schema}, "nullable": true}
	}
	out := map[string]any{}
	for k, v := range schema {
		out[k] = v
	}
	out["nullable"] = true
	return out
}
