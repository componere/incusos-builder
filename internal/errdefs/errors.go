package errdefs

import "errors"

// ErrConfig marks an invalid build configuration, including an oversized
// seed tar detected at splice time. Exit code 3.
var ErrConfig = errors.New("invalid config")

// ErrDecrypt marks a SOPS decryption failure after a top-level sops key
// was detected. Exit code 4.
var ErrDecrypt = errors.New("decryption failed")

// ErrFetch marks a failure acquiring or reading an update-server
// artifact, including GPT-probe drift on the acquired image. Exit code 5.
var ErrFetch = errors.New("acquisition failed")

// ErrOutput marks a failure writing a built artifact (image stream,
// rescue media, or publication). Exit code 6.
var ErrOutput = errors.New("output write failed")

// ErrVersionNotFound marks a release-resolution failure (unknown pin,
// empty channel, missing image, or a requested application the update
// does not carry). Exit code 5.
var ErrVersionNotFound = errors.New("version not found")
