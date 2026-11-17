# -*- coding: utf-8 -*-
import paramiko
from paramiko import SSHClient, AutoAddPolicy
import re
from flask import Flask, request, jsonify
import os

app = Flask(__name__)

# 配置SFTP服务器信息
SFTP_HOST = "10.146.25.10"
SFTP_PORT = 3014
SFTP_USERNAME = "mysftp"
SFTP_PASSWORD = "EversecA123098"
SFTP_LOCAL_PATH = "/data/sftp/tmp/rule/"


def sftp_download(remote_path):
    """
    使用SFTP下载文件
    :param remote_path: 远程文件路径
    :return: 下载结果
    """
    try:
        # 使用正则表达式解析URL
        pattern = r"sftp://(?P<username>[^:]+):(?P<password>[^@]+)@(?P<ip>[^:]+):(?P<port>\d+)(?P<path>/[^?#]*)"
        match = re.match(pattern, remote_path)
        if match:
            # 提取各个部分
            sftp_username = match.group("username")
            sftp_password = match.group("password")
            sftp_host = match.group("ip")
            sftp_port = int(match.group("port"))
            remote_file_path = match.group("path")
            # 提取文件名
            file_name = os.path.basename(remote_file_path)

            if not os.path.exists(SFTP_LOCAL_PATH):  # 检查目录是否存在
                os.makedirs(SFTP_LOCAL_PATH)  # 创建目录（支持多级目录）
                os.chmod(SFTP_LOCAL_PATH, 0o777)

            # 创建 SSH 客户端
            ssh = SSHClient()
            ssh.set_missing_host_key_policy(AutoAddPolicy())  # 自动添加主机密钥
            # 连接到服务器
            ssh.connect(
                hostname=sftp_host,
                port=sftp_port,
                username=sftp_username,
                password=sftp_password,
            )

            # 打开SFTP会话
            sftp = ssh.open_sftp()

            # 检查远程文件是否存在
            try:
                sftp.stat(remote_file_path)
            except IOError:
                raise IOError("远程文件 {} 不存在！".format(remote_file_path))

            # 在这里可以执行文件传输等操作
            sftp.get(remote_file_path, os.path.join(
                SFTP_LOCAL_PATH, file_name))

            # 关闭 SFTP 和 SSH 连接
            sftp.close()
            ssh.close()

            return True
        else:
            return False

    except paramiko.AuthenticationException:
        raise ValueError("连接失败：用户名或密码错误！")
    except paramiko.SSHException as ssh_err:
        raise ConnectionError("SSH 连接失败：{}".format(ssh_err))
    except Exception as e:
        print("下载失败: {}".format(e))
        return False

# 测试可以正常下载


@app.route("/downloadSftp", methods=["POST"])
def download_file():
    """
    SFTP下载接口
    请求参数：
        - url: 远程文件路径
    """
    try:
        data = request.json
        remote_path = data.get("url")

        if not remote_path:
            return jsonify({"code": 400, "msg": "缺少必要的参数"})

        result = sftp_download(remote_path)
        if result:
            return jsonify({"code": 200, "msg": "文件下载成功"}), 200
        else:
            return jsonify({"code": 400, "error": "文件下载失败"}), 500

    except IOError as e:
        return jsonify({"code": 400, "error": str(e)}), 404
    except ValueError as e:
        return jsonify({"code": 400, "error": str(e)}), 401
    except ConnectionError as e:
        return jsonify({"code": 400, "error": str(e)}), 502
    except Exception as e:
        return jsonify({"code": 400, "error": str(e)}), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8018, debug=False)
