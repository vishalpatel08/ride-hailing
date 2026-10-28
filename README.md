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

## Postman API flow

Create a Postman environment or collection with `baseUrl` set to `http://localhost:8080`. Set `userId`, `driverId`, and `rideId` from the `id` values returned by the corresponding create/book requests. For JSON requests, select **Body → raw → JSON** (or add `Content-Type: application/json`).

### 1. Create a user

- Method: `POST`
- URL: `{{baseUrl}}/users`
- Body:

```json
{
  "name": "Asha",
  "phone": "9000000001"
}
```

Save the returned `id` as `userId`.

### 2. Create a driver

- Method: `POST`
- URL: `{{baseUrl}}/drivers`
- Body:

```json
{
  "name": "Dev",
  "phone": "9000000002",
  "cabType": "HATCHBACK",
  "location": { "lat": 19.076, "lng": 72.8777 }
}
```

Save the returned `id` as `driverId`.

### 3. (Optional) Create a coupon

Create coupons before booking if you want to apply one. `maxDiscount` is in integer paise.

- Method: `POST`
- URL: `{{baseUrl}}/coupons`
- Body:

```json
{
  "code": "WELCOME",
  "type": "PERCENT",
  "value": 10,
  "maxDiscount": 5000
}
```

### 4. Book a ride

Omit `couponCode` when not using a coupon. Save the returned `id` as `rideId`.

- Method: `POST`
- URL: `{{baseUrl}}/rides`
- Body:

```json
{
  "userId": "{{userId}}",
  "pickup": { "lat": 19.076, "lng": 72.8777 },
  "drop": { "lat": 19.1, "lng": 72.9 },
  "cabType": "HATCHBACK",
  "couponCode": "WELCOME"
}
```

### 5. End the ride

The response includes distance and base, discount, and final fares in paise and formatted rupees.

- Method: `POST`
- URL: `{{baseUrl}}/rides/{{rideId}}/end`
- Body:

```json
{
  "drop": { "lat": 19.1, "lng": 72.9 }
}
```

### 6. Read completed ride history

- Method: `GET`
- URL: `{{baseUrl}}/users/{{userId}}/rides`
- Params: `status` = `COMPLETED`

Driver history is `GET {{baseUrl}}/drivers/{{driverId}}/rides` with the same optional `status` parameter.

### Optional requests

- Cancel an ongoing ride: `POST {{baseUrl}}/rides/{{rideId}}/cancel` (no body).
- Update a driver's location: `PUT {{baseUrl}}/drivers/{{driverId}}/location`, body `{"lat":19.1,"lng":72.9}`.
- Delete a coupon: `DELETE {{baseUrl}}/coupons/WELCOME`.

All monetary response fields ending in `Paise`, plus coupon `maxDiscount`, use integer paise.

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
