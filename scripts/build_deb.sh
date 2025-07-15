#!/bin/bash
# 功能：自动构建amd64/arm64双架构deb包
# 用法：./build_deb.sh <版本号> <维护者信息>
set -ex
if [ $# -lt 2 ]; then
  echo "用法: $0 <版本号> \"维护者姓名 <邮箱>\""
  exit 1
fi

VERSION=$1
MAINTAINER=$2

# 创建临时构建目录
mkdir -p /tmp/assismgr_{amd64,arm64}/DEBIAN
mkdir -p /tmp/assismgr_{amd64,arm64}/usr/sbin
mkdir -p /tmp/assismgr_{amd64,arm64}/usr/www/assismgr
mkdir -p /tmp/assismgr_{amd64,arm64}/etc/assismgr
mkdir -p /tmp/assismgr_{amd64,arm64}/etc/assismgr
mkdir -p /tmp/assismgr_{amd64,arm64}/usr/lib/systemd/system/multi-user.target.wants/

# 复制文件（假设当前目录为项目根目录）
cp out/assismgr-linux-amd64 /tmp/assismgr_amd64/usr/sbin/assismgr
cp out/assismgr-linux-arm64 /tmp/assismgr_arm64/usr/sbin/assismgr
cp -r public/* /tmp/assismgr_amd64/usr/www/assismgr/
cp -r public/* /tmp/assismgr_arm64/usr/www/assismgr/
cp HaPerfMonitor_config.json /tmp/assismgr_arm64/etc/assismgr/
cp HaPerfMonitor_config.json /tmp/assismgr_amd64/etc/assismgr/

# 生成control文件
for ARCH in amd64 arm64; do
  cat > /tmp/assismgr_${ARCH}/DEBIAN/control <<EOF
Package: assismgr
Version: ${VERSION}
Architecture: ${ARCH}
Maintainer: ${MAINTAINER}
Depends: systemd (>= 240), libc6 (>= 2.31)
Description: Assistant Manager Service with web interface
EOF

cat > /tmp/assismgr_${ARCH}/usr/lib/systemd/system/assismgr.service <<EOF

[Unit]
Description=Assistant Manager Service
After=network.target

[Service]
User=root
Type=simple
WorkingDirectory=/usr/www/assismgr
ExecStart=/usr/sbin/assismgr -s /usr/www/assismgr -c /etc/assismgr/HaPerfMonitor_config.json
Restart=on-failure
RestartSec=30s

[Install]
WantedBy=multi-user.target
EOF
ln -sf ../assismgr.service /tmp/assismgr_${ARCH}/usr/lib/systemd/system/multi-user.target.wants/assismgr.service
done

# 生成postinst脚本（服务注册）



# 构建deb包
dpkg-deb --build --root-owner-group /tmp/assismgr_amd64 ./out/assismgr_v${VERSION}_amd64.deb
dpkg-deb --build --root-owner-group /tmp/assismgr_arm64 ./out/assismgr_v${VERSION}_arm64.deb

# 清理临时文件
# rm -rf /tmp/assismgr_{amd64,arm64}
echo "构建完成: assismgr_${VERSION}_amd64.deb 和 assismgr_${VERSION}_arm64.deb"