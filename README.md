# Notiferry

Notiferry is a tiny, stateless Telegram HTTP relay. **It has no authentication:** keep its port host-local or behind a trusted, authenticated proxy.

## Quick start

Create a bot with [BotFather](https://t.me/BotFather), add it to the destination chat, then send it a message and call `https://api.telegram.org/bot<TOKEN>/getUpdates` to find the chat ID. (Remove any webhook first.) Forum topics use their topic/message-thread ID.

### Docker Compose

Put the included `compose.yaml`, `notiferry.yaml`, and a local `.env` in the
same directory. Create `notiferry.yaml` with your chat IDs (this is a complete,
valid example):

```yaml
listen: :8080
default_target: ops
targets:
  ops:
    chat_id: "-1001234567890"
    topic_id: 42
  phone:
    chat_id: "123456789"
```

Create `.env` in that directory and put the bot token on this line:

```dotenv
NOTIFERRY_TELEGRAM_BOT_TOKEN=123456:replace-with-your-real-token
```

Docker Compose automatically reads `.env` and the existing `environment`
mapping in `compose.yaml` passes that value to Notiferry. From the same
directory, start and inspect it:

```sh
docker compose up -d
docker compose ps
docker compose logs --tail=50 notiferry
```

Send a notification, then stop the service when finished:

```sh
curl -X POST localhost:8080/v1/notify \
  -H 'content-type: application/json' \
  -d '{"text":"hello"}'
docker compose down
```

Do not commit `.env` or put a real token in source control. The service has no
authentication; keep port 8080 bound to trusted host-local access as shown in
`compose.yaml` (`127.0.0.1:8080:8080`) and do not expose it more broadly.

For direct Docker, export the token and run:

```sh
export NOTIFERRY_TELEGRAM_BOT_TOKEN='123:secret'
docker run --rm -p 127.0.0.1:8080:8080 \
  -e NOTIFERRY_TELEGRAM_BOT_TOKEN \
  -v "$PWD/notiferry.yaml:/notiferry.yaml:ro" \
  ghcr.io/omerktn/notiferry:latest --config /notiferry.yaml
```

For a binary, run `notiferry --config notiferry.yaml`; edit the file and send
`SIGHUP` to reload it. The token is environment-only.

```sh
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"text":"hello"}'
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"target":"phone","text":"hello"}'
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"text":"<b>hello</b>","format":"html"}'
```

`GET /health/live` and `/health/ready` are available; the image healthcheck uses readiness. No queues or persistence are involved: success means Telegram accepted every chunk.
