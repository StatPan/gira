#!/bin/sh
set -eu

repo="StatPan/gira"
install_dir="${GIRA_INSTALL_DIR:-"$HOME/.local/bin"}"
version="${GIRA_VERSION:-latest}"

need() {
	if ! command -v "$1" >/dev/null 2>&1; then
		printf '%s\n' "install.sh: missing required command: $1" >&2
		exit 1
	fi
}

download() {
	url="$1"
	out="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$out"
	elif command -v wget >/dev/null 2>&1; then
		wget -q "$url" -O "$out"
	else
		printf '%s\n' "install.sh: missing required command: curl or wget" >&2
		exit 1
	fi
}

latest_version() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest" |
			sed 's#.*/##'
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "https://github.com/${repo}/releases/latest" |
			sed -n 's/.*href="\/StatPan\/gira\/releases\/tag\/\([^"]*\)".*/\1/p' |
			sed -n '1p'
	else
		printf '%s\n' "install.sh: missing required command: curl or wget" >&2
		exit 1
	fi
}

case "$(uname -s)" in
	Linux) os="linux" ;;
	Darwin) os="darwin" ;;
	*)
		printf '%s\n' "install.sh: unsupported OS: $(uname -s)" >&2
		exit 1
		;;
esac

case "$(uname -m)" in
	x86_64 | amd64) arch="amd64" ;;
	arm64 | aarch64) arch="arm64" ;;
	*)
		printf '%s\n' "install.sh: unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
esac

need sed
need tar
need mktemp
need mkdir
need chmod
need mv
need awk

if [ "$version" = "latest" ]; then
	version="$(latest_version)"
	if [ -z "$version" ] || [ "$version" = "latest" ]; then
		printf '%s\n' "install.sh: could not resolve latest release version" >&2
		exit 1
	fi
fi

name="gira_${version}_${os}_${arch}"
archive="${name}.tar.gz"
base_url="https://github.com/${repo}/releases/download/${version}"
tmpdir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup EXIT HUP INT TERM

download "${base_url}/${archive}" "${tmpdir}/${archive}"

checksum_file=""
for candidate in checksums.txt SHA256SUMS sha256sums.txt; do
	if download "${base_url}/${candidate}" "${tmpdir}/${candidate}" >/dev/null 2>&1; then
		checksum_file="${tmpdir}/${candidate}"
		break
	fi
done

if [ -n "$checksum_file" ]; then
	awk -v archive="$archive" '$2 == archive || $2 == "*" archive { print }' "$checksum_file" >"${tmpdir}/checksums.match"
	if [ ! -s "${tmpdir}/checksums.match" ]; then
		printf '%s\n' "install.sh: checksum asset does not include ${archive}" >&2
		exit 1
	fi
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$tmpdir" && sha256sum -c checksums.match)
	elif command -v shasum >/dev/null 2>&1; then
		(cd "$tmpdir" && shasum -a 256 -c checksums.match)
	else
		printf '%s\n' "install.sh: checksum asset exists but sha256sum/shasum is unavailable" >&2
		exit 1
	fi
fi

tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"
if [ ! -f "${tmpdir}/${name}/gira" ]; then
	printf '%s\n' "install.sh: release archive did not contain ${name}/gira" >&2
	exit 1
fi

mkdir -p "$install_dir"
chmod 0755 "${tmpdir}/${name}/gira"
target_tmp="${install_dir}/.gira.tmp.$$"
rm -f "$target_tmp"
mv "${tmpdir}/${name}/gira" "$target_tmp"
mv "$target_tmp" "${install_dir}/gira"

printf 'installed gira %s to %s/gira\n' "$version" "$install_dir"
case ":$PATH:" in
	*":$install_dir:"*) ;;
	*)
		printf 'add %s to PATH before running gira\n' "$install_dir"
		;;
esac
