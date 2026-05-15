# Offline Access

Cloudfuse includes an offline access feature in the `file_cache` component that allows reads and writes to continue against the local cache when cloud storage is temporarily unreachable. This document describes the feature, its configuration, and its consistency implications.

## How It Works

When cloud storage becomes unavailable, Cloudfuse continues to serve file operations from the local file cache. Reads return cached data. Writes are accepted and held in the local cache. When connectivity is restored, all pending changes are written to cloud storage.

## Enabling and Disabling

Offline access is **enabled by default**. To disable it — causing Cloudfuse to block local file access whenever cloud storage is unreachable — set the `block-offline-access` flag in your `file_cache` configuration:

```yaml
file_cache:
  block-offline-access: true
```

When `block-offline-access: true`, any filesystem operation that requires a cloud storage
 connection will fail with an error when offline, which is the stricter, previous behavior.

## Improving Offline Functionality with Attribute Cache Tuning

When offline, Cloudfuse cannot refresh file metadata from cloud storage. The `attr_cache` component caches this metadata locally. By default its timeout is **120 seconds (2 minutes)**, after which Cloudfuse attempts to revalidate metadata against cloud storage.

To extend the window during which cached metadata remains valid — reducing the chance of stale-metadata errors while offline — raise the `timeout-sec` value under `attr_cache`:

```yaml
attr_cache:
  timeout-sec: 3600   # cache metadata for 1 hour; adjust to your needs
```

Using a longer timeout makes it more likely that metadata will be available offline.

## Consistency Considerations

> **Read this section carefully before using offline access in a multi-client or shared-storage environment.**

### Eventual Consistency and Last-Writer-Wins

Cloudfuse only supports **eventual consistency**, using **last-writer-wins** semantics. Data is written to cloud storage when a file is *closed*, not when it is written. This means that, under normal operation, there is already a window during which cloud storage does not reflect the latest local changes.

For this reason, we strongly recommend only connecting to each container with a single client, or splitting the container into separate prefixes, each accessed by a single client using the **subdirectory** option.

### Offline Access Makes the Consistency Window Indefinitely Long

The offline access feature is **permissive by design**: it keeps local access open for as long as the client is disconnected. This means:

- A client may hold unsynchronized writes in its local cache indefinitely — for hours, days, or longer — until it reconnects.
- When the client reconnects, those writes will be uploaded to cloud storage.
- If another client has written to the same objects during that time, ***last-writer-wins semantics may cause the offline client's stale data to overwrite the newer data***.

One particularly unpleasant example:

1. A Cloudfuse client loses its network connection, then renames a directory while offline (this is allowed if all directory contents are cached).
2. Using another client, the contents of the source directory are updated.
3. Once the offline client reconnects, it **deletes all the objects within the source directory** and writes stale copies of those objects to the destination directory.

### Recommendation

**We never recommend concurrent access to the same objects from multiple clients.** The offline access feature *further increases the risk of consistency issues* in such configurations. If you must use multiple clients, be aware that:

1. A client returning from an extended offline period may overwrite changes made by other clients during that time.
2. There is no built-in conflict detection or merge — the last synchronization wins, regardless of content freshness.

Use offline access only in single client, prefix-separated client, or read-only scenarios to minimize the risk of conflicting writes.
