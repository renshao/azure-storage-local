# Azure Storage Local

A lightweight, fast Azure Storage Emulator written in Go. Drop-in replacement for Azurite for local development — focused on being simple, fast, and minimal.

Currently supports **Azure Queue Storage** APIs. Blob and Table support planned for later.

## Quick Start

```bash
go build -o azure-storage-local.exe .
./azure-storage-local.exe
```

The emulator starts two servers:
- **Queue API**: `http://127.0.0.1:10001/devstoreaccount1` — Azure SDK compatible
- **Web UI**: `http://127.0.0.1:10011` — Browser-based queue inspector

## Connection String

Use this connection string with any Azure Storage SDK:

```
DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;QueueEndpoint=http://127.0.0.1:10001/devstoreaccount1;
```

## Supported Queue APIs

| Operation | Method | Endpoint |
|-----------|--------|----------|
| List Queues | GET | `/{account}?comp=list` |
| Create Queue | PUT | `/{account}/{queue}` |
| Delete Queue | DELETE | `/{account}/{queue}` |
| Get Queue Metadata | GET | `/{account}/{queue}?comp=metadata` |
| Set Queue Metadata | PUT | `/{account}/{queue}?comp=metadata` |
| Put Message | POST | `/{account}/{queue}/messages` |
| Get Messages | GET | `/{account}/{queue}/messages` |
| Peek Messages | GET | `/{account}/{queue}/messages?peekonly=true` |
| Delete Message | DELETE | `/{account}/{queue}/messages/{id}?popreceipt=...` |
| Clear Messages | DELETE | `/{account}/{queue}/messages` |
| Update Message | PUT | `/{account}/{queue}/messages/{id}?popreceipt=...&visibilitytimeout=...` |

## Features

- **Message visibility timeout** — messages become invisible after dequeue
- **Message TTL** — messages auto-expire after their time-to-live
- **Pop receipts** — required for delete/update operations
- **Dequeue count** tracking
- **Web UI** — browse queues and messages in your browser
- **Single binary** — no external dependencies

## License

See [LICENSE](LICENSE) file.