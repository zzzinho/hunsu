#!/bin/sh
printf '{"type":"item.completed","item":{"type":"agent_message","text":"%s"}}\n{"type":"turn.completed"}\n' "$(cat)"
