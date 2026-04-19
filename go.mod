module git.pootis.network/hh-resume-auto-boost

go 1.25.0

require (
	github.com/imroc/req/v3 v3.57.0
	go.nhat.io/cookiejar v0.3.0
	golang.org/x/net v0.53.0
)

// quic-go has to be downgraded to v0.58.0 until https://github.com/imroc/req/issues/482 gets resolved.
replace github.com/quic-go/quic-go => github.com/quic-go/quic-go v0.58.0

require (
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/bool64/ctxd v1.2.1 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/icholy/digest v1.1.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/refraction-networking/utls v1.8.2 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)
