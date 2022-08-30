package hedge

import future.keywords.in

default allow = false

# name must be foo
named_foo {
  input.name == "foo"
}

# deprecated must be missing or false
not_deprecated {
  not input.deprecated == true
}

no_signature {
  not input.signature
}

signers := ["key1", "key2", "compromised-key"]
signed_by_trusted_key {
  some key in signers
  input.signature.keyFingerprint == key
}

allow {
  named_foo
  not_deprecated
  no_signature
}

allow {
  named_foo
  not_deprecated
  signed_by_trusted_key
}

deny {
  input.signature.keyFingerprint == "compromised-key"
}
