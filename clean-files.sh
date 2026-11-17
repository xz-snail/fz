#!/bin/bash

# 清理 /data/sftp/tmp/log 目录中超过2天的文件
# 每天凌晨2点执行清理
# 0 2 * * * find /data/sftp/tmp/log -type f -mtime +2 -delete
find /data/sftp/tmp/log -type f -mtime +2 -exec rm -f {} \;