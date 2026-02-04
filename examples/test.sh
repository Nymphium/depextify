#!/bin/bash

# コメント: ここは無視されるべき行
TARGET_DIR="/var/tmp/backup"
NOW=$(date +%Y%m%d)

# 関数定義
log_msg() {
    echo "[INFO] $1" | tee -a app.log
}

# ディレクトリ作成と論理演算子 (mkdir, touch)
if [ ! -d "$TARGET_DIR" ]; then
    mkdir -p "$TARGET_DIR" && touch "$TARGET_DIR/.lock"
fi

# パイプとxargs、コマンド置換 (find, xargs, rm, wc)
FILE_COUNT=$(find . -name "*.tmp" | wc -l)

if [ "$FILE_COUNT" -gt 0 ]; then
    log_msg "Cleaning up $FILE_COUNT files..."
    find . -name "*.tmp" -print0 | xargs -0 rm -f
else
    # 外部コマンド呼び出し (curl, jq)
    RESPONSE=$(curl -s https://api.example.invalid/status)
    echo "$RESPONSE" | jq '.status'
fi

# 最後に通知 (notify-send)
notify-send "Task Finished" "Backup check complete"