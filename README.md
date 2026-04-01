# Azure Storage Lite

A lightweight, fast Azure Storage Emulator written in Go. Drop-in replacement for Azurite for local development — focused on being simple, fast, and minimal.

Currently supports **Azure Queue Storage** and **Azure Blob Storage** APIs. Table support planned for later.

## Quick Start

```bash
go build -o azure-storage-lite.exe .
./azure-storage-lite.exe
```

The emulator starts three servers:
- **Blob API**: `http://127.0.0.1:10000/devstoreaccount1` — Azure SDK compatible
- **Queue API**: `http://127.0.0.1:10001/devstoreaccount1` — Azure SDK compatible
- **Web UI**: `http://127.0.0.1:10003` — Browser-based storage inspector

## Connection String

Use this connection string with any Azure Storage SDK:

```
DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;QueueEndpoint=http://127.0.0.1:10001/devstoreaccount1;
```

## Supported Blob APIs

| Operation | Method | Endpoint |
|-----------|--------|----------|
| List Containers | GET | `/{account}?comp=list` |
| Create Container | PUT | `/{account}/{container}?restype=container` |
| Delete Container | DELETE | `/{account}/{container}?restype=container` |
| List Blobs | GET | `/{account}/{container}?restype=container&comp=list` |
| Put Blob | PUT | `/{account}/{container}/{blob}` |
| Get Blob | GET | `/{account}/{container}/{blob}` |
| Get Blob Metadata | GET | `/{account}/{container}/{blob}?comp=metadata` |

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

- **Blob Storage** — create containers, upload/download blobs, list with delimiter support
- **Queue Storage** — full message lifecycle with visibility timeout and TTL
- **Web UI** — tabbed browser for queues and blobs with auto-refresh
- **Single binary** — no external dependencies
- **In-memory** — fast, no disk I/O

## License

See [LICENSE](LICENSE) file.