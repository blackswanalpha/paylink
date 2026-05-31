module github.com/paylink/mpesa-adapter

go 1.25.7

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/paylink/paylink-chain v0.0.0
	github.com/prometheus/client_golang v1.23.2
	github.com/redis/go-redis/v9 v9.20.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// The lVM wire format + crypto are reused byte-exact via paylink-chain/pkg/lvm. Go's internal/
// rule blocks importing paylink-chain/internal/* from this module, so we depend on the chain
// module (its public pkg/lvm) and resolve it from the monorepo checkout. Same pattern as
// linkmint-backend/proof-validator (work03).
replace github.com/paylink/paylink-chain => ../../paylink-chain
