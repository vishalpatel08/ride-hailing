# Ride Hailing Service

A small Go 1.22+ HTTP backend for booking, matching, pricing, ending, and cancelling rides. It uses only the Go standard library.

## Run

```sh
go run ./cmd/server
```

The server listens on `:8080`. Matching defaults to nearest-first; set `DRIVER_MATCHING_STRATEGY=rating` to use highest-rated-first (`nearest` is also accepted). The in-memory data is reset when the process stops.

Validate the project with:

```sh
go vet ./...
gofmt -l $(rg --files -g '*.go')
go test -race ./...
```

## API flow

Create a user and driver:

```sh
curl -sS -X POST localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Asha","phone":"9000000001"}'

curl -sS -X POST localhost:8080/drivers \
  -H 'Content-Type: application/json' \
  -d '{"name":"Dev","phone":"9000000002","cabType":"HATCHBACK","location":{"lat":19.076,"lng":72.8777}}'
```

Copy the returned user `id` into `USER_ID`. Book a ride and copy its returned ride `id` into `RIDE_ID`:

```sh
curl -sS -X POST localhost:8080/rides \
  -H 'Content-Type: application/json' \
  -d '{"userId":"USER_ID","pickup":{"lat":19.076,"lng":72.8777},"drop":{"lat":19.1,"lng":72.9},"cabType":"HATCHBACK"}'
```

End the ride at its actual destination; the response contains distance and the paise/rupee fare breakdown:

```sh
curl -sS -X POST localhost:8080/rides/RIDE_ID/end \
  -H 'Content-Type: application/json' \
  -d '{"drop":{"lat":19.1,"lng":72.9}}'
```

Read completed ride history:

```sh
curl -sS 'localhost:8080/users/USER_ID/rides?status=COMPLETED'
```

Coupons are optional. `maxDiscount` and all returned monetary `*Paise` values are integer paise:

```sh
curl -sS -X POST localhost:8080/coupons \
  -H 'Content-Type: application/json' \
  -d '{"code":"WELCOME","type":"PERCENT","value":10,"maxDiscount":5000}'

curl -sS -X POST localhost:8080/rides \
  -H 'Content-Type: application/json' \
  -d '{"userId":"USER_ID","pickup":{"lat":19.076,"lng":72.8777},"drop":{"lat":19.1,"lng":72.9},"cabType":"HATCHBACK","couponCode":"WELCOME"}'
```

Cancellation is available through `POST /rides/RIDE_ID/cancel`. Driver location updates use `PUT /drivers/DRIVER_ID/location` with `{"lat":19.1,"lng":72.9}`. Both user and driver history routes accept an optional `status` query parameter.

## Architecture

```text
HTTP handlers (strict JSON, status mapping)
        |
RideService ---- CouponService
   |       |          |
   |       +--- pricing + surge strategy
   +----------- matching strategy
        |
Repository interfaces
        |
Mutex-protected in-memory repositories
```

The handler layer decodes requests, calls services, and serializes responses. Services coordinate booking, lifecycle, and fare operations. Pricing and matching strategies are injected; repositories isolate storage.

## Design decisions

- Driver selection ranks candidates in the configured radius and reserves the first available driver atomically. Hatchback requests can be upgraded to a sedan; sedan requests are never downgraded.
- Ride fares use the requested cab category. Surge is recorded when a ride is booked, then applied before coupon discount when the ride ends.
- Money is represented as `int64` paise. Distance and tier calculations use kilometers and round the calculated fare once.
- HTTP JSON bodies are size-limited and reject unknown fields. Errors map to consistent HTTP statuses. SIGINT and SIGTERM trigger graceful shutdown.

## Assumptions & Tradeoffs

- **Storage:** In-memory storage was chosen to fit the assignment time limit. Repository interfaces make MySQL or Redis-backed implementations swappable. A relational sketch would use `users(id, name, phone)`, `drivers(id, name, phone, rating, status, cab_type, lat, lng)`, `rides(id, user_id, driver_id, requested_cab_type, assigned_cab_type, pickup_lat, pickup_lng, drop_lat, drop_lng, status, start_time, end_time, distance_km, base_fare_paise, discount_paise, final_fare_paise, surge_multiplier, coupon_code, cancellation_fee_paise)`, and `coupons(code, type, value, max_discount_paise, active)`. Production transactions/constraints should enforce one ongoing ride per user and driver. Redis GEO can index driver locations for proximity searches, with driver availability maintained alongside it.
- **Distance:** Haversine gives straight-line distance; there is no road network, traffic, or route detour calculation.
- **Pricing:** Rates are marginal per-kilometer slabs; the minimum fare is applied after slab totals are calculated.
- **Upgrade:** An upgraded hatchback request is charged at the requested hatchback rate, not the assigned sedan rate.
- **Concurrency:** Driver reservation is an atomic per-driver AVAILABLE→ON_RIDE compare-and-set (implemented under the driver repository mutex), with no service-wide booking lock. Ride persistence atomically checks for another ongoing ride by the same user; losing a reservation attempt releases the driver. Repository-level mutexes protect in-memory maps.
- **Matching scale:** Radius filtering is an O(n) scan over available drivers. A production deployment should use a spatial index such as geohash or H3 (or Redis GEO) to narrow candidates.
- **Active rides:** A user can have at most one ongoing ride, and a driver can serve at most one ongoing ride. Completed and cancelled rides do not occupy either resource.
- **Surge inputs:** Current rides and available drivers are counted from repositories; pending/unmatched open requests are supplied through an injectable counter. This synchronous assignment has no persistent request queue, so the server currently supplies no separate open-request counter.
- **Next with more time:** Add persistent transactions and migrations, request validation and authentication, idempotency keys, pagination, observability, road-distance integration, a durable unmatched-request queue, and broader failure-injection/integration tests.
