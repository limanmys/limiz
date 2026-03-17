package signing

// EmbeddedPublicKey is set at compile time via -ldflags.
// It contains the base64-encoded contents of keys/plugin-signing.pub.
// Both metric plugins and data plugins are verified using this key.
var EmbeddedPublicKey string
