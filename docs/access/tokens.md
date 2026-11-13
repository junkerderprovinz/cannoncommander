# API tokens

Scripts, WebDAV clients and other programs sign in with an API token instead of your password. A token can be limited to reading.

![The API tokens card with one token, backup-skript, that may read and write, and the button Create a token.](../assets/screenshots/account-tokens.png)

## Create a token {#create}

1. Open **Settings > Account & security** and scroll to **API tokens**.
2. Click **Create a token**.
3. Give it a **Name**, such as `backup-script`.
4. Pick the **Rights**: Read, or Read and write.
5. Pick when it **Expires**: 30 days, 1 year or never.
6. Enter your password and click **Create**.
7. Copy the token. It is shown only this once.

![The window Create a token: name, rights Read or Read and write, expires in 30 days, 1 year or never, and the current password.](../assets/screenshots/token-add.png)

## Use a token {#use}

Send it in the `Authorization` header:

```sh
curl -H "Authorization: Bearer <token>" https://files.example.com/api/v1/me
```

The full API description is served at `/api/v1/openapi.yaml`.

For [WebDAV](../integrations/webdav.md) with two-factor sign-in on, use the token as the password.

## What a token may do {#rights}

A token acts as you, with your role, your area and your [access rules](rules.md). A read-only token can look and download, nothing else. Tokens never get a [terminal](../integrations/terminal.md).

## Revoke a token {#revoke}

Click the token and choose **Revoke token**. Programs using it are turned away at once. An admin can see and revoke the tokens of every user.
