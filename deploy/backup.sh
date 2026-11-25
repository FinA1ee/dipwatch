#!/bin/sh
# Daily Postgres backup on the server, keeping the newest 7. Installed in the
# deploy user's crontab by deploy.sh. Restore into a scratch database with:
#   gunzip -c backups/dipwatch-<date>.sql.gz | docker exec -i dipwatch-postgres-1 psql -U dipwatch -d <scratch>
set -eu
cd "$(dirname "$0")"
mkdir -p backups
out="backups/dipwatch-$(date +%Y%m%d-%H%M).sql.gz"
# --clean --if-exists: a restore replaces the tables it contains.
docker exec dipwatch-postgres-1 pg_dump -U dipwatch -d dipwatch --clean --if-exists | gzip -9 > "$out.tmp"
mv "$out.tmp" "$out"
ls -1t backups/dipwatch-*.sql.gz | tail -n +8 | xargs -r rm -f
echo "backup: $out ($(du -h "$out" | cut -f1))"
