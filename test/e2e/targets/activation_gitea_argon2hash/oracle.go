package activation_gitea_argon2hash

import "encoding/base64"

// directInvocationPassword is the password used for the direct invocation probe.
const directInvocationPassword = "test-password"

// directInvocationSaltRaw is the raw salt bytes used for the direct invocation probe.
var directInvocationSaltRaw = []byte("test-salt-16byte")

// directInvocationSaltB64 is the base64-encoded salt for the invoke payload.
var directInvocationSaltB64 = base64.StdEncoding.EncodeToString(directInvocationSaltRaw)
