# SUDP: Safe UDP

# Tests

## Unit

```bash
go test ./... -timeout 60s -v -cover -race
```

## E2E

### Parameters

Currently, there are 3 scenarios for **client-server interaction** and 10 **network parameters**.

Client-server interactions:
- `first-message.yml` - measuring the time to the first ping (comparing sudp and tcp).
Env: [`CLIENTS_COUNT`, `REQUEST_SIZE`]
- `messages.yml` - sending and receiving many random messages (comparing sudp and tcp).
Env: [`CLIENTS_COUNT`, `REQUEST_SIZE`, `REQUEST_COUNT`]
- `http.yml` - regular http request.
Env: [`CLIENTS_COUNT`]

Network parameters (in `/scenarios`):
- `ideal.env` - verify basic correctness
- `lan.env` - stable wired connection
- `wi-fi.env` - typical Wi-Fi
- `mobile.env` - mobile (4G / average 5G)
- `congested.env` - tests retransmissions & backoff
- `poor.env` - bad connection
- `satellite-like.env` - high latency, low loss
- `asymmetric-routing.env` - mobile uplinks
- `server-under-load.env` - server-side buffering behavior
- `high-loss.env` - 10% packet loss

### Run the tests

Firstly:
```bash
cd e2e
```

Template:

```bash
{ENV_PARAM=value} \
docker compose \
-f {client-server interaction} \
--env-file scenarios/{network parameters} \
up --build
```

Example:

```bash
CLIENTS_COUNT=10 REQUEST_SIZE=4096 REQUEST_COUNT=100 \
docker compose \
-f messages.yml \
--env-file scenarios/wi-fi.env \
up --build
```