#!/usr/bin/env python3
"""
Analyze cloudfuse size tracker logs to track file sizes and detect discrepancies.

This script processes log files to:
1. Track the size of each file in the filesystem
2. Detect when files are created, modified, renamed, or deleted
3. Identify discrepancies in size calculations
4. Monitor sync operations and epoch changes
"""

import re
import sys
from typing import Dict, Optional
from dataclasses import dataclass
from datetime import datetime


@dataclass
class FileInfo:
    """Information about a file in the filesystem."""
    size: Optional[int]  # None means unknown size
    last_updated: str  # Timestamp of last update

    def __repr__(self):
        size_str = str(self.size) if self.size is not None else "UNKNOWN"
        return f"FileInfo(size={size_str}, last_updated={self.last_updated})"


class SizeTrackerAnalyzer:
    """Analyzer for size tracker logs."""

    def __init__(self):
        self.files: Dict[str, FileInfo] = {}
        self.total_delta = 0
        self.last_sync_total = None
        self.discrepancies = []
        self.line_number = 0
        self.timestamp = ""
        self.epochChanged = False
        self.firstSync = True

    def parse_timestamp(self, line: str) -> Optional[str]:
        """Extract timestamp from log line."""
        match = re.match(r'^(\w+ \w+ \d+ \d+:\d+:\d+\.\d+ \w+ \d+)', line)
        return match.group(1) if match else None

    def handle_add(self, line: str):
        """Handle SizeTracker::Add log entries."""
        match = re.search(r'SizeTracker::Add : (-?\d+)', line)
        if match:
            delta = int(match.group(1))
            self.total_delta += delta
            print(f"[{self.line_number}] [{self.timestamp}] Add delta: {delta:+d}, cumulative delta: {self.total_delta:+d}")

    def handle_copy_from_file(self, line: str):
        """Handle SizeTracker::CopyFromFile log entries."""
        debug_match = re.search(r'SizeTracker::CopyFromFile : (.+?) Add\((.+?)\)', line)

        if debug_match:
            filepath = debug_match.group(1).strip()
            size_info = debug_match.group(2)

            # Parse size_info which
            # e.g. "84535181-4096" (new size - old size)
            if size_info.find("-") < 1:
                delta = int(size_info)
                # Update file info
                if filepath in self.files:
                    self.files[filepath].size += delta
                    self.files[filepath].last_updated = self.timestamp
                print(f"[{self.line_number}] [{self.timestamp}] CopyFromFile (OLD STYLE): '{filepath}' -> delta={delta:+d}")
                return
            parts = size_info.split('-')
            new_size = int(parts[0])
            old_size = int(parts[1])
            delta = new_size - old_size

            # Check if file exists and if the old size matches
            if filepath in self.files:
                tracked_size = self.files[filepath].size
                if tracked_size is not None and tracked_size != old_size:
                    discrepancy = f"[{self.line_number}] [{self.timestamp}] CopyFromFile size mismatch for '{filepath}': expected old_size={tracked_size}, got old_size={old_size}"
                    print(f"  ⚠️  {discrepancy}")
                    self.discrepancies.append(discrepancy)

            # Update file info
            self.files[filepath] = FileInfo(size=new_size, last_updated=self.timestamp)
            print(f"[{self.line_number}] [{self.timestamp}] CopyFromFile: '{filepath}' -> size={new_size} (delta={delta:+d})")

    def handle_delete_file(self, line: str):
        """Handle SizeTracker::DeleteFile log entries."""
        debug_match = re.search(r'SizeTracker::DeleteFile : (.+?) Add\((-\d+)\)', line)

        if debug_match:
            filepath = debug_match.group(1).strip()
            delta = int(debug_match.group(2))

            # Check if file exists
            if filepath in self.files:
                tracked_size = self.files[filepath].size
                if tracked_size is not None and delta != -tracked_size:
                    discrepancy = f"[{self.line_number}] [{self.timestamp}] DeleteFile size mismatch for '{filepath}': expected delta={-tracked_size}, got delta={delta}"
                    print(f"  ⚠️  {discrepancy}")
                    self.discrepancies.append(discrepancy)

                # Remove from tracking
                del self.files[filepath]
                print(f"[{self.line_number}] [{self.timestamp}] DeleteFile: '{filepath}' (delta={delta:+d})")
            else:
                print(f"[{self.line_number}] [{self.timestamp}] DeleteFile: '{filepath}' (delta={delta:+d}) [file not tracked]")

    def handle_rename_file(self, line: str):
        """Handle SizeTracker::RenameFile log entries."""
        match = re.search(r'SizeTracker::RenameFile : (.+?)->(.+)$', line)
        if match:
            src = match.group(1).strip()
            dst = match.group(2).strip()

            # Transfer file info from src to dst
            if src in self.files:
                self.files[dst] = self.files[src]
                del self.files[src]
                print(f"[{self.line_number}] [{self.timestamp}] RenameFile: '{src}' -> '{dst}'")
            else:
                # File not tracked, but still record the rename
                self.files[dst] = FileInfo(size=None, last_updated=self.timestamp)
                print(f"[{self.line_number}] [{self.timestamp}] RenameFile: '{src}' -> '{dst}' [src not tracked]")

    def handle_sync(self, line: str):
        """Handle SizeTracker::sync log entries."""
        # Check for epoch change
        epoch_match = re.search(r'epoch changed \(local=(\d+) -> file=(\d+)\) — discarding delta (-?\d+)', line)
        if epoch_match:
            local_epoch = int(epoch_match.group(1))
            file_epoch = int(epoch_match.group(2))
            discarded_delta = int(epoch_match.group(3))
            print(f"\n{'='*80}")
            print(f"[{self.line_number}] [{self.timestamp}] 🔄 EPOCH CHANGE: local={local_epoch} -> file={file_epoch}, discarding delta={discarded_delta:+d}")
            if self.total_delta != discarded_delta:
                discrepancy = f"[{self.line_number}] [{self.timestamp}] Delta mismatch: expected delta={self.total_delta}. Discarded delta is off by {discarded_delta-self.total_delta}!"
                print(f"  ⚠️  {discrepancy}")
                self.discrepancies.append(discrepancy)
            print(f"{'='*80}\n")
            self.total_delta = 0  # Reset delta after epoch change
            self.epochChanged = True
            self.firstSync = False
            return

        # Normal sync pattern: "old_total + delta = new_total"
        sync_match = re.search(r'SizeTracker::sync : (\d+) \+ (-?\d+) = (\d+)', line)
        if sync_match:
            old_total = int(sync_match.group(1))
            delta = int(sync_match.group(2))
            new_total = int(sync_match.group(3))

            print(f"\n{'='*80}")
            print(f"[{self.line_number}] [{self.timestamp}] 📊 SYNC: {old_total} + {delta:+d} = {new_total}")

            # Verify the old total is correct
            if self.last_sync_total is not None and old_total != self.last_sync_total:
                epochDisclaimer = ""
                if self.epochChanged:
                    epochDisclaimer = "(updated by audit)"
                    self.epochChanged = False
                discrepancy = f"[{self.line_number}] [{self.timestamp}] Sync size mismatch: {old_total} != {self.last_sync_total} (off by {old_total - self.last_sync_total})! {epochDisclaimer}"
                print(f"  ⚠️  {discrepancy}")
                self.discrepancies.append(discrepancy)

            # Compare with tracked delta
            if not self.firstSync and self.total_delta != delta:
                discrepancy = f"[{self.line_number}] [{self.timestamp}] Sync delta mismatch: tracked={self.total_delta:+d}, sync={delta:+d}, difference={self.total_delta - delta:+d}"
                print(f"  ⚠️  {discrepancy}")
                self.discrepancies.append(discrepancy)

            print(f"{'='*80}\n")

            # Reset for next sync period
            self.last_sync_total = new_total
            self.total_delta = 0
            self.firstSync = False

    def process_line(self, line: str):
        """Process a single log line."""
        self.line_number += 1

        self.timestamp = self.parse_timestamp(line)
        if not self.timestamp:
            return

        # Route to appropriate handler based on log message
        if 'SizeTracker::Add :' in line and 'journal.go' in line:
            self.handle_add(line)
        elif 'SizeTracker::CopyFromFile' in line:
            self.handle_copy_from_file(line)
        elif 'SizeTracker::DeleteFile' in line:
            self.handle_delete_file(line)
        elif 'SizeTracker::RenameFile' in line:
            self.handle_rename_file(line)
        elif 'SizeTracker::sync' in line:
            self.handle_sync(line)

    def print_summary(self):
        """Print summary of analysis."""
        print("\n" + "="*80)
        print("ANALYSIS SUMMARY")
        print("="*80)
        print(f"Total lines processed: {self.line_number}")
        print(f"Total files tracked: {len(self.files)}")
        print(f"Pending delta (since last sync): {self.total_delta:+d}")

        if self.discrepancies:
            print(f"\n⚠️  Found {len(self.discrepancies)} discrepancies:")
            for i, discrepancy in enumerate(self.discrepancies, 1):
                print(f"  {i}. {discrepancy}")
        else:
            print("\n✅ No discrepancies found!")

        # Show some file statistics
        known_size_files = [f for f in self.files.values() if f.size is not None]
        unknown_size_files = [f for f in self.files.values() if f.size is None]

        print(f"\nFiles with known size: {len(known_size_files)}")
        print(f"Files with unknown size: {len(unknown_size_files)}")

        if known_size_files:
            total_size = sum(f.size for f in known_size_files)
            print(f"Total size of tracked files: {total_size:,} bytes ({total_size / (1024**3):.2f} GB)")


def main():
    """Main entry point."""
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <log_file>")
        print(f"Example: {sys.argv[0]} combined.log")
        sys.exit(1)

    log_file = sys.argv[1]

    print(f"Analyzing log file: {log_file}")
    print("="*80 + "\n")

    analyzer = SizeTrackerAnalyzer()

    try:
        with open(log_file, 'r', encoding='utf-8', errors='ignore') as f:
            for line in f:
                analyzer.process_line(line.rstrip('\n'))
    except FileNotFoundError:
        print(f"Error: File '{log_file}' not found")
        sys.exit(1)
    except Exception as e:
        print(f"Error processing log file: {e}")
        sys.exit(1)

    analyzer.print_summary()


if __name__ == "__main__":
    main()
