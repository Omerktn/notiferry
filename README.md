# Notiferry

Notiferry is a tiny, stateless Telegram HTTP relay. **It has no authentication:** keep its port host-local or behind a trusted, authenticated proxy.

## Quick start

Create a bot with [BotFather](https://t.me/BotFather), add it to the destination chat, then send it a message and call `https://api.telegram.org/bot<TOKEN>/getUpdates` to find the chat ID. (Remove any webhook first.) Forum topics use their topic/message-thread ID.

### Docker Compose

Run these commands from the cloned repository root, which is the directory that
contains `compose.yaml`. Copy the example config, then edit it:

```sh
cp notiferry.example.yaml notiferry.yaml
${EDITOR:-vi} notiferry.yaml
```

Set the bot token and target chat IDs in `notiferry.yaml`:

```yaml
listen: :8080
telegram_bot_token: "123456:replace-with-your-real-token"
default_target: ops
targets:
  ops:
    chat_id: "-1001234567890"
    topic_id: 42
  phone:
    chat_id: "123456789"
```

Docker Compose mounts the host-side `./notiferry.yaml` inside the container as
`/notiferry.yaml`. Always edit the host-side file beside `compose.yaml`, not the
path inside the container. The local file is ignored by Git because it contains
credentials; keep it private.

To keep the token outside YAML instead, copy `.env.example` to `.env` and set
`NOTIFERRY_TELEGRAM_BOT_TOKEN`. The environment variable overrides the YAML
value.

From that same directory, start and inspect the service:

```sh
docker compose up -d
docker compose ps
docker compose logs --tail=50 notiferry
```

Send a notification, then stop the service when finished:

```sh
curl -X POST localhost:3333/v1/notify \
  -H 'content-type: application/json' \
  -d '{"text":"hello"}'
docker compose down
```

The service has no authentication; keep port 3333 bound to trusted host-local
access as shown in `compose.yaml` (`127.0.0.1:3333:8080`) and do not expose it
more broadly.

For direct Docker, build the image, export the token, and run:

```sh
docker build -t notiferry:local .
export NOTIFERRY_TELEGRAM_BOT_TOKEN='123:secret'
docker run --rm -p 127.0.0.1:3333:8080 \
  -e NOTIFERRY_TELEGRAM_BOT_TOKEN \
  -v "$PWD/notiferry.yaml:/notiferry.yaml:ro" \
  notiferry:local --config /notiferry.yaml
```

For a binary, run `notiferry --config notiferry.yaml`; edit the file and send
`SIGHUP` to reload targets. Changing the listen address or bot token requires a
restart.

```sh
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"text":"hello"}'
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"target":"phone","text":"hello"}'
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"text":"<b>hello</b>","format":"html"}'
```

`GET /health/live` and `/health/ready` are available; the image healthcheck uses readiness. No queues or persistence are involved: success means Telegram accepted every chunk.
