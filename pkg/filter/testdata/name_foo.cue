// name must be foo
name: "foo"

// deprecated must be missing or false
deprecated?: false

// Signature must be missing (:thisisfine:) or signed by key1/key2
signature?: {
    keyFingerprint: "key1" | "key2"
}
