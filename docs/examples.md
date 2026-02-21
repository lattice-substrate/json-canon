# Examples

## Canonicalization examples

### Basic key sorting

Input:

```json
{"z":3,"a":1}
```

Command:

```bash
echo '{"z":3,"a":1}' | ./jcs-canon canonicalize
```

Output:

```json
{"a":1,"z":3}
```

### Recursive sorting

Input:

```json
{"b":[{"z":1,"a":2}],"a":3}
```

Output:

```json
{"a":3,"b":[{"a":2,"z":1}]}
```

### Number formatting boundaries

```bash
echo '1e20' | ./jcs-canon canonicalize   # 100000000000000000000
echo '1e21' | ./jcs-canon canonicalize   # 1e+21
echo '1e-7' | ./jcs-canon canonicalize   # 1e-7
```

## Verification examples

### Canonical input

```bash
printf '%s' '{"a":1}' | ./jcs-canon verify --quiet -
echo $?  # 0
```

### Non-canonical ordering

```bash
printf '%s' '{"b":1,"a":2}' | ./jcs-canon verify --quiet -
echo $?  # 2
```

### `-0` profile rejection

```bash
printf '%s\n' '-0' | ./jcs-canon verify --quiet -
echo $?  # 2
```

This fails by design in the strict profile and is not normalized.

### Trailing whitespace rejected

```bash
printf '{"a":1}\n' | ./jcs-canon verify --quiet -
echo $?  # 2
```

## UTF-16 sort divergence example

This demonstrates RFC 8785 key sorting behavior:

Input keys:

- `\uD800\uDC00` (U+10000)
- `\uE000` (U+E000)

Command:

```bash
echo '{"\uE000":1,"\uD800\uDC00":2}' | ./jcs-canon canonicalize
```

Output order places U+10000 key first due to UTF-16 code-unit ordering.
