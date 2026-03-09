# Project Notes

## KOSPI Master (`kospi_code.mst`)

- Download URL: `https://new.real.download.dws.co.kr/common/master/kospi_code.mst.zip`
- Treat `kospi_code.mst` as a business-date-sensitive master file, not as permanent static metadata.
- The file includes dynamic or date-sensitive fields such as:
  - previous-day volume
  - previous-day market cap
  - base price
  - trading halt / warning flags
  - financial snapshot fields such as net income, ROE, and base year-month
- Do not use `base_date` / `기준년월` as the file freshness date. It is a financial reference period, not the master file generation date.

## Go Implementation Rules

- Relevant code:
  - `go/internal/domesticstock/market_status.go`
  - `go/cmd/main.go`
- The Go code must cache the KOSPI master by business date.
- `KOSPI_MASTER_CACHE_FILE` is the base path, not the final dated filename.
- The resolved master cache path format is:
  - `.cache/kospi_code.<YYYYMMDD>.mst`
- The resolved JSON sidecar path format is:
  - `.cache/kospi_code.<YYYYMMDD>.json`
- The resolved actual PBR cache path format is:
  - `.cache/kospi_actual_pbr.<YYYYMMDD>.json`
- The preferred `YYYYMMDD` source is the `market-time` API response when available.
- Do not rely on a simple 24-hour TTL for `kospi_code.mst`.
- If the same business date file already exists, reuse it.
- If the business date changes, download and store a new dated file.

## JSON Sidecar

- When the dated `.mst` cache is loaded in Go, also create a JSON sidecar for human inspection.
- The JSON sidecar currently exports the fields that the Go implementation actually uses:
  - `code`
  - `name`
  - `market_cap`
  - `net_income`
  - `roe`
  - `base_date`
- The JSON also includes:
  - `business_date`
  - `generated_at`
  - `source_path`
  - `record_count`
  - `fields`

## PBR Notes

- KOSPI weighted PBR in Go uses:
  - the dated KOSPI master cache for constituent selection by market cap
  - a separate actual PBR cache file controlled by `KOSPI_ACTUAL_PBR_CACHE_FILE`
- `KOSPI_ACTUAL_PBR_CACHE_FILE` is also a base path, not the final dated filename.
- Preserve the separation between:
  - business-date-scoped KOSPI master cache
  - business-date-scoped actual PBR cache files

## If You Modify This Logic

- Preserve business-date-scoped master cache filenames.
- Preserve JSON sidecar generation.
- Prefer `market-time` derived business dates over plain wall-clock dates when possible.
- Do not collapse the master cache back to a single undated file.
