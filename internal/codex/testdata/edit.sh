#!/bin/sh
cat <<'JSON'
{"type":"thread.started","thread_id":"t1"}
{"type":"item.started","item":{"type":"file_change","changes":[{"path":"/proj/a.html","kind":"update"}]}}
{"type":"item.completed","item":{"type":"agent_message","text":"고쳤습니다"}}
{"type":"turn.completed","usage":{}}
JSON
