# Notiferry

Notiferry is a tiny, stateless Telegram HTTP relay. **It has no authentication:** keep its port host-local or behind a trusted, authenticated proxy.

## Quick start

Create a bot with [BotFather](https://t.me/BotFather), add it to the destination chat, then send it a message and call `https://api.telegram.org/bot<TOKEN>/getUpdates` to find the chat ID. (Remove any webhook first.) Forum topics use their topic/message-thread ID.

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

```sh
export NOTIFERRY_TELEGRAM_BOT_TOKEN='123:secret'
docker run --rm -p 127.0.0.1:8080:8080 \
  -e NOTIFERRY_TELEGRAM_BOT_TOKEN \
  -v "$PWD/notiferry.yaml:/notiferry.yaml:ro" \
  ghcr.io/omerktn/notiferry:latest --config /notiferry.yaml
```

The same setup can be run with `docker compose up` using the included `compose.yaml` (which also publishes only to localhost). For a binary, run `notiferry --config notiferry.yaml`; edit the file and send `SIGHUP` to reload it. The token is environment-only.

```sh
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"text":"hello"}'
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"target":"phone","text":"hello"}'
curl -X POST localhost:8080/v1/notify -H 'content-type: application/json' -d '{"text":"<b>hello</b>","format":"html"}'
```

`GET /health/live` and `/health/ready` are available; the image healthcheck uses readiness. No queues or persistence are involved: success means Telegram accepted every chunk.
