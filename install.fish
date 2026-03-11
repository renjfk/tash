#!/usr/bin/env fish
# Usage: curl -fsSL https://raw.githubusercontent.com/renjfk/tash/main/install.fish | fish

set -l repo "renjfk/tash"
set -l install_dir "$HOME/.local/bin"

set -l os (uname -s)
set -l arch (uname -m)

switch $os
    case Darwin Linux
        # supported
    case '*'
        echo "tash: unsupported OS: $os" >&2
        exit 1
end

switch $arch
    case x86_64 amd64
        set arch x86_64
    case arm64 aarch64
        set arch arm64
    case '*'
        echo "tash: unsupported architecture: $arch" >&2
        exit 1
end

set -l binary tash_"$os"_"$arch"

# resolve latest release tag via GitHub API
set -l tag (curl -fsSL https://api.github.com/repos/$repo/releases/latest | string match -r '"tag_name":\s*"([^"]+)"' | tail -1)

if test -z "$tag"
    echo "tash: failed to resolve latest release tag" >&2
    exit 1
end

set -l ver (string replace 'v' '' $tag)
set -l base_url "https://github.com/$repo/releases/download/$tag"
set -l checksum_file tash_"$ver"_checksums.txt

echo "tash: downloading $binary ($tag)..."

mkdir -p $install_dir
set -l tmp_dir (mktemp -d)

# download binary and checksums
curl -fsSL $base_url/$binary -o $tmp_dir/tash; or begin
    echo "tash: download failed" >&2
    rm -rf $tmp_dir
    exit 1
end
curl -fsSL $base_url/$checksum_file -o $tmp_dir/checksums.txt; or begin
    echo "tash: checksum download failed" >&2
    rm -rf $tmp_dir
    exit 1
end

# verify checksum
set -l expected (string match -r "^\\S+" (grep $binary $tmp_dir/checksums.txt))

if test -z "$expected"
    echo "tash: checksum not found for $binary" >&2
    rm -rf $tmp_dir
    exit 1
end

set -l actual
if command -q sha256sum
    set actual (sha256sum $tmp_dir/tash | string split ' ')[1]
else if command -q shasum
    set actual (shasum -a 256 $tmp_dir/tash | string split ' ')[1]
else
    echo "tash: sha256sum or shasum required" >&2
    rm -rf $tmp_dir
    exit 1
end

if test "$actual" != "$expected"
    echo "tash: checksum mismatch" >&2
    echo "  expected: $expected" >&2
    echo "  got:      $actual" >&2
    rm -rf $tmp_dir
    exit 1
end

mv $tmp_dir/tash $install_dir/tash
rm -rf $tmp_dir
chmod +x $install_dir/tash

echo "tash: installed to $install_dir/tash (checksum verified)"
echo ""
echo "  Set your API key and run:"
echo "    export ANTHROPIC_API_KEY=your-key"
echo "    tash init"
