# Single sign-on with OIDC

CannonCommander signs in through any OpenID Connect provider you run.

## Set it up {#setup}

1. In your provider, create an application for CannonCommander with this redirect address:

    ```text
    https://files.example.com/api/auth/oidc/callback
    ```

2. Note its client ID, client secret and issuer address.
3. Add them to `/config/config.yaml`:

    ```yaml
    auth:
      methods:
        oidc:
          enabled: true
          clientId: "cannoncommander"
          clientSecret: "the secret from your provider"
          issuerUrl: "https://auth.example.com/application/o/cannoncommander/"
          adminGroup: "cannoncommander-admins"
    ```

4. Restart the container.

The sign-in page now shows a button for it:

![The sign-in page with the Pocket ID button below the password sign-in.](../assets/screenshots/sign-in-oidc.png)

## The button {#button}

The button carries the name and mark of your provider. CannonCommander tells Pocket ID, authentik, Authelia, Keycloak, Kanidm and Zitadel apart by their addresses, so most setups need nothing more. Two keys change it:

| Key | What it does |
|---|---|
| `name` | The name on the button, such as `Family login`. Empty is the provider's own name. |
| `provider` | The kind of provider when it is not recognised: `pocket-id`, `authentik`, `authelia`, `keycloak`, `kanidm`, `zitadel` or `custom`. |

## All keys {#keys}

| Key | What it does |
|---|---|
| `enabled` | Switches the sign-in on. |
| `issuerUrl`, `clientId`, `clientSecret` | The application at your provider. |
| `name`, `provider` | The button, see above. |
| `scopes` | What CannonCommander asks for. Empty takes your provider's usual scopes. |
| `userIdentifier` | The claim that becomes the user name. Empty takes your provider's, usually `preferred_username`. |
| `groupsClaim` | The claim with the user's groups. Empty takes your provider's, usually `groups`. |
| `adminGroup`, `userGroups` | Who becomes an admin and who gets in, see below. |
| `caFile` | A PEM file with the certificate authority of a provider that runs on your own certificate. |

`disableVerifyTLS` is not possible: name the provider's certificate authority in `caFile` instead.

## Who gets in {#who}

- A user who signs in for the first time is created on the spot, as a user.
- Members of `adminGroup` become admins.
- `userGroups` limits sign-in to the groups you list.

## Keep the secret out of the file {#env}

The client ID and secret can come from the variables `CC_OIDC_CLIENT_ID` and `CC_OIDC_CLIENT_SECRET` instead.

## Behind a reverse proxy {#proxy}

Set `externalUrl` so the redirect address matches, see [Reverse proxy](reverse-proxy.md).
