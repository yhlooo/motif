#!/bin/bash

set -e

# 确定版本
version="${1:-latest}"

# 确定操作系统
os="$(uname -s)"
case "${os}" in
  "Linux")
    os="linux"
  ;;
  "Darwin")
    os="darwin"
  ;;
  *)
    echo -e "\033[31mError: unsupported os ${os}\033[0m"
    exit 1
  ;;
esac

# 确定 CPU 架构
arch="$(uname -m)"
case "${arch}" in
  "x86_64")
    arch="amd64"
  ;;
  "arm64")
  ;;
  *)
    echo -e "\033[31mError: unsupported arch ${arch}\033[0m"
    exit 1
  ;;
esac

# 确定安装位置
install_dir="${INSTALL_DIR:-${HOME}/.local/bin}"

# 确定 URL
if [[ "${version}" == "latest" ]]; then
  url="https://github.com/yhlooo/motif/releases/latest/download/motif-${os}-${arch}.tar.gz"
else
  url="https://github.com/yhlooo/motif/releases/download/${version}/motif-${os}-${arch}.tar.gz"
fi

echo -e "\033[34minstalling motif (${version}, ${os}/${arch}) to '${install_dir}/motif' ...\033[0m"
echo -e "\033[34mdownload from ${url}\033[0m"

# 下载、解压
mkdir -p "${install_dir}"
curl -fsSL "${url}" | tar -C "${install_dir}" -xzv motif
chmod +x "${install_dir}/motif"

# 检查 PATH
if [[ ":${PATH}:" != *":${install_dir}:"* ]]; then
  echo -e "\033[33mWarning: install dir '${install_dir}' not in \$PATH\033[0m"
  case "${SHELL}" in
    *"/zsh")
      echo -e "\033[33m         run \`echo 'export PATH=\"${install_dir}:\${PATH}\"' >> ~/.zshrc && source ~/.zshrc\` to add\033[0m"
    ;;
    *"/bash")
      echo -e "\033[33m         run \`echo 'export PATH=\"${install_dir}:\${PATH}\"' >> ~/.bashrc && source ~/.bashrc\` to add\033[0m"
    ;;
    *)
      echo -e "\033[33m         run \`export PATH=\"${install_dir}:\${PATH}\"\` to add\033[0m"
    ;;
  esac
fi
