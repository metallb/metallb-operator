#!/usr/bin/env python3

import os
import re
import subprocess
import argparse
import sys

GO_MOD = "go.mod"
DOCKERFILE = "Dockerfile"


def base_dir() -> str:
    """Return the absolute path to the project root directory."""
    return os.path.join(os.path.dirname(os.path.realpath(__file__)), "..")


def read_file(file_name: str) -> list[str]:
    """Read a file from the project root and return its lines as a list."""
    try:
        with open(os.path.join(base_dir(), file_name), "r") as f:
            return f.readlines()
    except IOError as e:
        raise RuntimeError(f"Failed to read {file_path}: {str(e)}")


def write_file(file_name: str, lines: list[str]):
    """Write a list of lines to a file in the project root."""
    try:
        with open(os.path.join(base_dir(), file_name), "w") as f:
            for line in lines:
                f.write(line)
    except IOError as e:
        raise RuntimeError(f"Failed to write to {file_path}: {str(e)}")


def get_go_version(lines: list[str]) -> tuple[str, str]:
    """Extract the Go version and toolchain version from go.mod lines."""
    re_go = re.compile(r"^go\s+(\d+\.\d+(?:\.\d+)?)")
    re_toolchain = re.compile(r"^toolchain\sgo(\d+\.\d+\.\d+)")

    go_version = ""
    go_toolchain = ""
    for line in lines:
        if go_version != "" and go_toolchain != "":
            break
        match = re_go.search(line)
        if match:
            go_version = match.group(1)
            continue
        match = re_toolchain.search(line)
        if match:
            go_toolchain = match.group(1)
            continue
    return (go_version, go_toolchain)


def get_docker_go_version(lines: list[str]) -> str:
    """Extract the Go version version from Dockerfile lines."""
    re_go = re.compile(r"docker.io/golang:(\d+\.\d+(?:\.\d+)?)")

    go_version = ""
    for line in lines:
        if go_version != "":
            break
        match = re_go.search(line)
        if match:
            return match.group(1)
    return ""


def set_docker_go_version(lines: list[str], go_version: str):
    """Update the Go version in Dockerfile lines."""
    re_go = re.compile(r"docker.io/golang:(\d+\.\d+(?:\.\d+)?)")

    for idx in range(len(lines)):
        lines[idx] = re_go.sub(f"docker.io/golang:{go_version}", lines[idx])


def get_k8s_version(lines: list[str]) -> str:
    """Extract the Kubernetes version from go.mod lines."""
    # Prefer k8s.io/kubernetes as canonical version, fall back to k8s.io/api
    re_k8s_kubernetes = re.compile(r"k8s.io/kubernetes v(\d+)\.(\d+\.\d+)")
    re_k8s_api = re.compile(r"k8s.io/api v(\d+)\.(\d+\.\d+)")

    api_version = ""
    for line in lines:
        # Check for kubernetes version (preferred)
        match = re_k8s_kubernetes.search(line)
        if match:
            return match.group(2)
        # Check for api version (fallback)
        match = re_k8s_api.search(line)
        if match:
            api_version = match.group(2)

    return api_version


def set_go_version(lines: list[str], go_version: str, go_toolchain: str):
    """Update the Go version and toolchain in go.mod lines."""
    re_go = re.compile(r"^go\s+(\d+\.\d+(?:\.\d+)?)")
    re_toolchain = re.compile(r"^toolchain\sgo(\d+\.\d+\.\d+)")

    go_version_set = False
    go_toolchain_set = False
    go_line_idx = -1

    for idx in range(len(lines)):
        if go_version_set and go_toolchain_set:
            return
        match = re_go.search(lines[idx])
        if match:
            lines[idx] = f"go {go_version}\n"
            go_version_set = True
            go_line_idx = idx
            continue
        match = re_toolchain.search(lines[idx])
        if match:
            lines[idx] = f"toolchain go{go_toolchain}\n"
            go_toolchain_set = True
            continue

    if not go_version_set:
        # Should never happen
        raise Exception(f"invalid content, go version not set")
    if not go_toolchain_set:
        # Insert toolchain right after the go line
        lines.insert(go_line_idx + 1, f"toolchain go{go_toolchain}\n")


def set_k8s_version(lines: list[str], k8s_version: str):
    """Update all k8s.io component versions in go.mod lines."""
    re_k8s = re.compile(r"^(\s+)k8s.io/([\w-]+) v(\d+)\.(\d+\.\d+)($| .+$)")

    for idx in range(len(lines)):
        match = re_k8s.search(lines[idx])
        if match:
            leading_space = match.group(1)
            component = match.group(2)
            major_version = match.group(3)
            minor_version = match.group(4)
            trailer = match.group(5)
            lines[idx] = (
                f"{leading_space}k8s.io/{component} v{major_version}.{k8s_version}{trailer}\n"
            )


def go_mod_tidy():
    """Run 'go mod tidy' in the project root directory."""
    print(f"Running go mod tidy ...")
    result = subprocess.run(
        ["go", "mod", "tidy"], cwd=base_dir(), capture_output=True, text=True
    )
    if result.returncode != 0:
        print(f"ERROR: running 'go mod tidy': {result.stderr}", file=sys.stderr)
        sys.exit(1)


def update_docker_file(docker_filename: str, docker_go_version: str):
    """Update docker file go version."""
    print(f"Writing to file '{docker_filename}' ...")
    docker_lines = read_file(docker_filename)
    set_docker_go_version(docker_lines, docker_go_version)
    write_file(docker_filename, docker_lines)


def update_go_mod(filename: str, go_version: str, go_toolchain: str, k8s_version: str):
    """Update go mod go and k8s version."""
    print(f"Writing to file '{filename}' ...")
    lines = read_file(filename)
    set_go_version(lines, go_version, go_toolchain)
    set_k8s_version(lines, k8s_version)
    write_file(filename, lines)


def list_versions(filename: str, docker_filename: str):
    """List current versions"""
    docker_lines = read_file(docker_filename)
    current_docker_go_version = get_docker_go_version(docker_lines)
    print(f"{docker_filename} go version:\t'{current_docker_go_version}'")

    lines = read_file(filename)
    current_go_version, current_go_toolchain = get_go_version(lines)
    current_k8s_version = get_k8s_version(lines)
    print(f"{filename} go version:\t'{current_go_version}'")
    print(f"{filename} go toolchain:\t'{current_go_toolchain}'")
    print(f"{filename} k8s version:\t'{current_k8s_version}'")
    if current_go_version == "":
        print(f"ERROR: go version is not set in file {filename}, exiting")
        sys.exit(1)


def parse() -> dict[str, str]:
    """Parse and validate command line arguments."""
    parser = argparse.ArgumentParser()
    parser.add_argument("-l", "--list", action=argparse.BooleanOptionalAction)
    parser.add_argument(
        "-f", "--go-mod-filename", default=GO_MOD, help=f"Default: {GO_MOD}"
    )
    parser.add_argument(
        "-df", "--docker-filename", default=DOCKERFILE, help=f"Default: {DOCKERFILE}"
    )
    parser.add_argument("-dg", "--docker-go-version", default="", help="e.g. 1.24.11")
    parser.add_argument("-g", "--go-version", default="", help="e.g. 1.24.11")
    parser.add_argument("-k", "--k8s-version", default="", help="e.g. 34.3")
    args = parser.parse_args()

    if args.list:
        return args.__dict__

    validate(args)

    return args.__dict__


def validate(args: argparse.Namespace):
    """Validate command line arguments."""
    if not args.docker_go_version:
        args.docker_go_version = args.go_version
    path = os.path.join(base_dir(), args.go_mod_filename)
    if not os.path.isfile(path):
        print(f"ERROR: File {path} is not a valid file")
        sys.exit(1)
    path = os.path.join(base_dir(), args.docker_filename)
    if not os.path.isfile(path):
        print(f"ERROR: File {path} is not a valid file")
        sys.exit(1)
    if not re.match(r"^\d+\.\d+\.\d+$", args.go_version):
        print("ERROR: Must provide valid value for --go-version (e.g. 1.24.11)")
        sys.exit(1)
    if not re.match(r"^\d+\.\d+\.\d+$", args.docker_go_version):
        print("ERROR: Must provide valid value for --docker-go-version (e.g. 1.24.11)")
        sys.exit(1)
    if not re.match(r"^\d+\.\d+$", args.k8s_version):
        print("ERROR: Must provide valid value for --k8s-version (e.g. 34.3)")
        sys.exit(1)


def main():
    parsed = parse()
    shall_list = parsed["list"]
    filename = parsed["go_mod_filename"]
    docker_filename = parsed["docker_filename"]

    if shall_list:
        list_versions(filename, docker_filename)
        return

    docker_go_version = parsed["docker_go_version"]
    go_version = ".".join(parsed["go_version"].split(".")[:2])
    go_toolchain = parsed["go_version"]
    k8s_version = parsed["k8s_version"]

    print(f"Requested:")
    print(f"Dockerfile name:\t'{docker_filename}'")
    print(f"Dockerfile go version:\t'{docker_go_version}'")
    print(f"go mod file name:\t'{filename}'")
    print(f"go version:\t\t'{go_version}'")
    print(f"go toolchain:\t\t'{go_toolchain}'")
    print(f"k8s version:\t\t'{k8s_version}'")
    print("")

    update_docker_file(docker_filename, docker_go_version)
    update_go_mod(filename, go_version, go_toolchain, k8s_version)
    go_mod_tidy()

    print(
        f"\nAll commands executed successfully. Updated {filename} and {docker_filename} to:"
    )
    list_versions(filename, docker_filename)
    print("\nVerify and commit changes. Next, run:")
    print("```")
    print("make manifests; make bin; make bundle-release")
    print("```")
    print("And commit those modifications individually")


if __name__ == "__main__":
    main()
