package dump

// CouchDBDriver handles CouchDB dumps via the HTTP API.
// CouchDB has no CLI dump tools; all data access is through the REST API.
type CouchDBDriver struct{}

func (d *CouchDBDriver) Name() string { return "couchdb" }

// awkExtractDocs is an awk program that extracts "doc" objects from a CouchDB
// _all_docs JSON response. It reads the full response into memory, then walks
// through character-by-character, correctly handling quoted strings (including
// escape sequences) so that braces inside string values are never miscounted.
const awkExtractDocs = `{s=s $0}
END{
  n=length(s); i=1
  while (i <= n) {
    c = substr(s, i, 1)
    if (c == "\"") {
      j = i; i++
      while (i <= n) {
        if (substr(s, i, 1) == "\\") { i += 2; continue }
        if (substr(s, i, 1) == "\"") break
        i++
      }
      i++
      if (substr(s, j, i - j) == "\"doc\"" && substr(s, i, 1) == ":") {
        i++
        if (substr(s, i, 1) == "{") {
          d = 0; b = i
          while (i <= n) {
            cc = substr(s, i, 1)
            if (cc == "\"") {
              i++
              while (i <= n) {
                if (substr(s, i, 1) == "\\") { i += 2; continue }
                if (substr(s, i, 1) == "\"") break
                i++
              }
            } else if (cc == "{") d++
            else if (cc == "}") {
              d--
              if (d == 0) { print substr(s, b, i - b + 1); break }
            }
            i++
          }
          i++
        }
      }
      continue
    }
    i++
  }
}`

func (d *CouchDBDriver) DumpCommand(opts DumpOptions) []string {
	user := opts.User
	if user == "" {
		user = "admin"
	}

	curlAuth := ""
	if opts.PasswordEnv != "" {
		curlAuth = ` -u "$1:$` + opts.PasswordEnv + `"`
	}

	if opts.Database != "" && opts.Database != "all" {
		script := `set -e
BASE=http://localhost:5984
echo "%%DB:$2"
curl -sf` + curlAuth + ` "$BASE/$2/_all_docs?include_docs=true&attachments=true" | awk '` + awkExtractDocs + `'`
		return []string{"sh", "-c", script, "--", user, opts.Database}
	}

	script := `set -e
BASE=http://localhost:5984
for DB in $(curl -sf` + curlAuth + ` "$BASE/_all_dbs" | tr -d '"[]' | tr ',' '\n'); do
  case "$DB" in _*) continue;; esac
  echo "%%DB:$DB"
  curl -sf` + curlAuth + ` "$BASE/$DB/_all_docs?include_docs=true&attachments=true" | awk '` + awkExtractDocs + `'
done`
	return []string{"sh", "-c", script, "--", user}
}

func (d *CouchDBDriver) RestoreCommand(opts RestoreOptions) []string {
	user := opts.User
	if user == "" {
		user = "admin"
	}

	curlAuth := ""
	if opts.PasswordEnv != "" {
		curlAuth = ` -u "$1:$` + opts.PasswordEnv + `"`
	}

	script := `set -e
BASE=http://localhost:5984
DB=""
BATCH=""
COUNT=0
flush() {
  if [ "$COUNT" -gt 0 ] && [ -n "$DB" ]; then
    printf '{"docs":[%s],"new_edits":false}' "$BATCH" | \
      curl -sf` + curlAuth + ` -X POST "$BASE/$DB/_bulk_docs" -H "Content-Type: application/json" -d @- > /dev/null
  fi
  BATCH=""
  COUNT=0
}
while IFS= read -r LINE; do
  case "$LINE" in
    %%DB:*)
      flush
      DB="${LINE#%%DB:}"
      curl -sf` + curlAuth + ` -X DELETE "$BASE/$DB" > /dev/null 2>&1 || true
      curl -sf` + curlAuth + ` -X PUT "$BASE/$DB" > /dev/null
      ;;
    "") ;;
    *)
      if [ "$COUNT" -gt 0 ]; then
        BATCH="$BATCH,$LINE"
      else
        BATCH="$LINE"
      fi
      COUNT=$((COUNT + 1))
      if [ "$COUNT" -ge 500 ]; then
        flush
      fi
      ;;
  esac
done
flush`
	return []string{"sh", "-c", script, "--", user}
}

func (d *CouchDBDriver) ReadyCommand(opts DumpOptions) []string {
	if opts.PasswordEnv != "" {
		user := opts.User
		if user == "" {
			user = "admin"
		}
		return []string{"sh", "-c", `curl -sf -u "$1:$` + opts.PasswordEnv + `" http://localhost:5984/_up`, "--", user}
	}
	return []string{"curl", "-sf", "http://localhost:5984/_up"}
}

func (d *CouchDBDriver) PreRestoreCommand(opts RestoreOptions) []string { return nil }

func (d *CouchDBDriver) FileExtension() string { return "jsonl" }

func (d *CouchDBDriver) Validate(labels map[string]string) error { return nil }
