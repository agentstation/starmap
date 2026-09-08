# Nested presence regression evidence

CSP3 also covers nested fields. The optional root-record prototype does not address these failures.

Ten round-trip cases lose explicit null values across five paths and two output formats. The parent test also fails. The known-zero JSON control passes. The package exits with status 1.

The tested paths are metadata knowledge cutoff, lineage parent, generation maximum tokens, pricing input, and the input token amount. JSON encoding turns an explicit null token amount into zero. YAML encoding omits that amount. Both formats omit the other tested null values.

The test decodes JSON, then encodes JSON or YAML. It does not test YAML input or inference billing. Preserve these limits when interpreting the result.

The compressed files retain the test, overlay map, and output. `verification.json` binds the source and counts. No production patch or task completion follows from this probe.
