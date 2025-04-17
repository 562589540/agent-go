## HTTPS 请求处理

## 信任代理 CA 证书 (用于 HTTPS 拦截)

为了让代理能够拦截和检查 HTTPS 流量（例如，替换 Google API 请求中的 API 密钥），客户端（如您的浏览器或 Go 程序）必须信任代理使用的自签名 CA 证书。否则，您会遇到证书错误。

代理首次运行时，会在其工作目录下的 `.goproxy-ca` 子目录中自动生成 CA 证书 (`goproxy_ca_cert.pem`) 和私钥 (`goproxy_ca_key.pem`)。

**警告：** 将自定义 CA 证书添加到系统的信任存储区具有安全风险。请仅在您完全信任此代理程序的来源并且了解相关风险的情况下执行此操作。在不使用代理时，建议移除此证书。

以下是在不同操作系统上添加信任的步骤：

### macOS

1. 打开"终端"应用程序。
2. 运行以下命令（需要管理员权限）：

    ```bash
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$(pwd)/.goproxy-ca/goproxy_ca_cert.pem"
    ```

    *(请确保命令中的路径指向您实际运行代理并生成证书的目录下的 `.goproxy-ca/goproxy_ca_cert.pem` 文件)*

### Windows

1. 以管理员身份打开命令提示符 (cmd) 或 PowerShell。
2. 运行以下命令：

    ```powershell
    certutil -addstore -f "ROOT" "%cd%\.goproxy-ca\goproxy_ca_cert.pem"
    ```

    *(请确保命令中的路径指向您实际运行代理并生成证书的目录下的 `.goproxy-ca\goproxy_ca_cert.pem` 文件)*
3. 或者，您可以通过图形界面操作：
    * 按 `Win + R`，输入 `certmgr.msc` 并回车。
    * 右键点击"受信任的根证书颁发机构" -> "所有任务" -> "导入..."。
    * 按照向导，浏览并选择 `.goproxy-ca/goproxy_ca_cert.pem` 文件。
    * 确保证书存储位置是"受信任的根证书颁发机构"。

### Linux (Debian/Ubuntu)

1. 打开终端。
2. 将证书复制到系统证书目录：

    ```bash
    sudo cp "$(pwd)/.goproxy-ca/goproxy_ca_cert.pem" /usr/local/share/ca-certificates/goproxy_ca.crt
    ```

3. 更新系统证书库：

    ```bash
    sudo update-ca-certificates
    ```

### Linux (Fedora/CentOS/RHEL)

1. 打开终端。
2. 将证书复制到信任源目录：

    ```bash
    sudo cp "$(pwd)/.goproxy-ca/goproxy_ca_cert.pem" /etc/pki/ca-trust/source/anchors/goproxy_ca.pem
    ```

3. 更新系统信任库：

    ```bash
    sudo update-ca-trust extract
    ```

### 特殊应用程序 (如 Firefox)

某些应用程序（例如 Firefox 浏览器）维护自己的独立证书信任库，不使用操作系统的信任库。对于这些应用程序，您可能需要单独导入 CA 证书。请查阅特定应用程序的文档了解如何导入自定义 CA 证书。

---

## 故障排查指南
