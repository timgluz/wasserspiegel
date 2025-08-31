# Trigger-Measurement

Trigger-Measurement is a command to trigger measurement collection tasks.

It is designed to work as Cron job.

## Prerequisites

- Go 1.23+
- Access to the Wasserspiegel API
- Account on [Render](https://render.com)

## How to run

- load environment variables from `.env` file

```bash
set -o allexport; source .env; set +o allexport

go run main.go
```
