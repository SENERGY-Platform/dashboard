# dashboard

Stores the users' dashboards and their widgets in MongoDB and serves them over an HTTP API on port 8080.

## Configuration

Environment variables, optionally from a `.env` file. An empty value counts as unset and gets the default.

| Env var | Default | Notes |
|---|---|---|
| `MONGO_URL` | `mongodb://localhost:27017` | Full connection string including scheme, passed to the driver unchanged. |
| `MONGO_USER` | empty | No authentication when empty. |
| `MONGO_PASSWORD` | empty | Required when `MONGO_USER` is set; masked wherever it is printed. |
| `MONGO_AUTH_SOURCE` | `admin` | Database the user is defined in. |
| `MONGO_DATABASE` | `dashboard` | Collection is `dashboards`. |
| `MIGRATE` | `true` | Runs the dashboard migrations (indices, `updatedAt`, `layout_mode`, `columns`) at startup. |
| `SYNC` | unset | `true` copies all dashboards from the legacy standalone mongo in `MONGO` into the configured database before startup. |
| `MONGO` | unset | Only with `SYNC=true`: `host:port` of the legacy standalone mongo, database `dashboard`, no authentication. |

When `MONGO_USER` is set, the credentials are built from `MONGO_USER`, `MONGO_PASSWORD` and `MONGO_AUTH_SOURCE` alone: they replace any user, password, `authSource` and `authMechanism` given in `MONGO_URL`. Keep credentials out of `MONGO_URL`; put them in `MONGO_USER` and `MONGO_PASSWORD`.

Startup fails unless an authenticated `listCollections` on `MONGO_DATABASE` succeeds within 10 seconds, and also when a migration or the sync fails.

## Tests

    go test ./...

The suite starts its own mongo through testcontainers and needs a docker daemon.

`TestInitDBAuthenticates` runs only without `-short` and when `MONGO_AUTH_TEST_URL`, `MONGO_AUTH_TEST_USER` and `MONGO_AUTH_TEST_PASSWORD` are set. The user and password are root credentials of a throwaway server with access control; the test creates and removes its own users and databases there. Example:

    docker run -d --rm --name dashboard-auth-test -p 127.0.0.1:27018:27017 -e MONGO_INITDB_ROOT_USERNAME=root -e MONGO_INITDB_ROOT_PASSWORD=rootpw mongo:8.2
    MONGO_AUTH_TEST_URL=mongodb://127.0.0.1:27018 MONGO_AUTH_TEST_USER=root MONGO_AUTH_TEST_PASSWORD=rootpw go test ./lib/ -run TestInitDBAuthenticates
    docker stop dashboard-auth-test
