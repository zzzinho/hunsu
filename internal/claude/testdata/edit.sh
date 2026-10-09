#!/bin/sh
cat <<'JSON'
{"type":"system","subtype":"init","session_id":"s1"}
{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"tool_use","name":"Edit"}}}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"file_path\":\"/proj/a.html\"}"}}}
{"type":"stream_event","event":{"type":"content_block_stop"}}
{"type":"result","subtype":"success","is_error":false,"result":"고쳤습니다"}
JSON
