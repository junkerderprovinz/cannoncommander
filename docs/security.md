# Security

CannonCommander can read, change and delete everything you map into it, so it deserves the same care as the Unraid web interface.

## The short checklist {#checklist}

1. [Set a password](getting-started/first-start.md#password) right after the install.
2. Turn on [two-factor sign-in](access/two-factor.md) for admins.
3. Reach it from outside only through a [reverse proxy](configuration/reverse-proxy.md) with https, or through a VPN.
4. Give people the [role and area](access/users.md) they need, not more.
5. Leave the [terminal](integrations/terminal.md) and [Unraid write access](configuration/unraid.md#write) off unless you need them.

## How it protects you {#how}

- **Sign-in.** Without a password CannonCommander opens for everyone who reaches it. Once an account has a password or a passkey, every visitor signs in. After five wrong passwords for one name from one address, that address waits 15 minutes.
- **The data folder is locked.** The database and `config.yaml` in `/config` cannot be opened through the file manager, not even by an admin, even when `/config` lies inside a share you mapped.
- **Users and rules.** Admins see everything. Everyone else stays inside an area of their own, and [access rules](access/rules.md) hide a folder or make it read only. The rules apply to the interface, WebDAV, share links and API tokens alike, and disk shares are for admins only.
- **API tokens** can be read only, and new tokens and passkeys ask for your password again. An admin can see and revoke the sessions, tokens and passkeys of every user.
- **Unraid write access** is off until you switch it on, because the emhttpd socket controls the whole array.
- **The terminal** is off until you switch it on, because it hands admins a shell with the rights of CannonCommander's user.
- **MCP keys** read until you allow writing, which asks for your password again. The server keeps only a fingerprint of each key, and an admin can take writing away or revoke any key.

## Report a security problem {#report}

Please report it privately through [GitHub's security advisories](https://github.com/junkerderprovinz/cannoncommander/security/advisories/new) rather than in a public issue. I will get back to you as soon as I can.
