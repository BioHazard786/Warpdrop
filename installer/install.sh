#!/bin/bash
#===============================================================================
#
#          FILE: install.sh
# 
#         USAGE: curl https://install.warpdrop.qzz.io | bash
#                 OR
#                wget -qO- https://install.warpdrop.qzz.io/install.sh | bash
# 
#   DESCRIPTION: WarpDrop CLI Installer Script.
#
#                This script installs the WarpDrop CLI into a specified prefix.
#                Default prefix = /usr/local/bin
#
#       OPTIONS: -p, --prefix "${INSTALL_PREFIX}"
#                      Prefix to install WarpDrop CLI into. Defaults to /usr/local/bin
#  REQUIREMENTS: bash, uname, tar/unzip, curl/wget, sudo (if not run
#                as root), install, mktemp, sha256sum/shasum/sha256
#
#          BUGS: ...hopefully not.  Please report.
#
#         NOTES: Homepage: https://github.com/BioHazard786/Warpdrop
#                  Issues: https://github.com/BioHazard786/Warpdrop/issues
#                  Donate: https://github.com/sponsor/BioHazard786
#
#        AUTHOR: Mohd Zaid (BioHazard786),
#
#       CREATED: 12/10/2025
#===============================================================================
set -o nounset                              # Treat unset variables as an error

#-------------------------------------------------------------------------------
# DEFAULTS
#-------------------------------------------------------------------------------
PREFIX="${PREFIX:-}"
ANDROID_ROOT="${ANDROID_ROOT:-}"

# Termux on Android has ${PREFIX} set which already ends with '/usr'
if [[ -n "${ANDROID_ROOT}" && -n "${PREFIX}" ]]; then
  INSTALL_PREFIX="${PREFIX}/bin"
else
  INSTALL_PREFIX="/usr/local/bin"
fi

#-------------------------------------------------------------------------------
# FUNCTIONS
#-------------------------------------------------------------------------------


#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  print_banner
#   DESCRIPTION:  Prints a banner
#    PARAMETERS:  none
#       RETURNS:  0
#-------------------------------------------------------------------------------
print_banner() {
  cat <<-'EOF'
=================================================
 ██╗    ██╗  █████╗  ██████╗  ██████╗  ██████╗  ██████╗   ██████╗  ██████╗
 ██║    ██║ ██╔══██╗ ██╔══██╗ ██╔══██╗ ██╔══██╗ ██╔══██╗ ██╔═══██╗ ██╔══██╗
 ██║ █╗ ██║ ███████║ ██████╔╝ ██████╔╝ ██║  ██║ ██████╔╝ ██║   ██║ ██████╔╝
 ██║███╗██║ ██╔══██║ ██╔══██╗ ██╔═══╝  ██║  ██║ ██╔══██╗ ██║   ██║ ██╔═══╝
 ╚███╔███╔╝ ██║  ██║ ██║  ██║ ██║      ██████╔╝ ██║  ██║ ╚██████╔╝ ██║
  ╚══╝╚══╝  ╚═╝  ╚═╝ ╚═╝  ╚═╝ ╚═╝      ╚═════╝  ╚═╝  ╚═╝  ╚═════╝  ╚═╝

 ██╗ ███╗   ██╗ ███████╗ ████████╗  █████╗  ██╗      ██╗      ███████╗ ██████╗
 ██║ ████╗  ██║ ██╔════╝ ╚══██╔══╝ ██╔══██╗ ██║      ██║      ██╔════╝ ██╔══██╗
 ██║ ██╔██╗ ██║ ███████╗    ██║    ███████║ ██║      ██║      █████╗   ██████╔╝
 ██║ ██║╚██╗██║ ╚════██║    ██║    ██╔══██║ ██║      ██║      ██╔══╝   ██╔══██╗
 ██║ ██║ ╚████║ ███████║    ██║    ██║  ██║ ███████╗ ███████╗ ███████╗ ██║  ██║
 ╚═╝ ╚═╝  ╚═══╝ ╚══════╝    ╚═╝    ╚═╝  ╚═╝ ╚══════╝ ╚══════╝ ╚══════╝ ╚═╝  ╚═╝                                                                                 
==================================================
EOF
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  print_help
#   DESCRIPTION:  Prints out a help message
#    PARAMETERS:  none
#       RETURNS:  0
#-------------------------------------------------------------------------------
print_help() {
  local help_header
  local help_message

  help_header="WarpDrop CLI Installer Script"
  help_message="Usage:
  -p INSTALL_PREFIX
      Prefix to install WarpDrop CLI into. Directory must already exist.
      Default = /usr/local/bin ('\${PREFIX}/bin' on Termux for Android)
  
  -h
      Prints this helpful message and exit."

  echo "${help_header}"
  echo ""
  echo "${help_message}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  print_message
#   DESCRIPTION:  Prints a message all fancy like
#    PARAMETERS:  $1 = Message to print
#                 $2 = Severity. info, ok, error, warn
#       RETURNS:  Formatted Message to stdout
#-------------------------------------------------------------------------------
print_message() {
  local message
  local severity
  local red
  local green
  local yellow
  local nc

  message="${1}"
  severity="${2}"
  red='\e[0;31m'
  green='\e[0;32m'
  yellow='\e[1;33m'
  nc='\e[0m'

  case "${severity}" in
    "info" ) echo -e "${nc}${message}${nc}";;
      "ok" ) echo -e "${green}${message}${nc}";;
   "error" ) echo -e "${red}${message}${nc}";;
    "warn" ) echo -e "${yellow}${message}${nc}";;
  esac


}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  make_tempdir
#   DESCRIPTION:  Makes a temp dir using mktemp if available
#    PARAMETERS:  $1 = Directory template
#       RETURNS:  0 = Created temp dir. Also prints temp file path to stdout
#                 1 = Failed to create temp dir
#                 20 = Failed to find mktemp
#-------------------------------------------------------------------------------
make_tempdir() {
  local template
  local tempdir
  local tempdir_rcode

  template="${1}.XXXXXX"

  if command -v mktemp >/dev/null 2>&1; then
    tempdir="$(mktemp -d -t "${template}")"
    tempdir_rcode="${?}"
    if [[ "${tempdir_rcode}" == "0" ]]; then
      echo "${tempdir}"
      return 0
    else
      return 1
    fi
  else
    return 20
  fi
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  determine_os
#   DESCRIPTION:  Attempts to determine host os using uname
#    PARAMETERS:  none
#       RETURNS:  0 = OS Detected. Also prints detected os to stdout
#                 1 = Unknown OS
#                 20 = 'uname' not found in path
#-------------------------------------------------------------------------------
determine_os() {
  local uname_out

  if command -v uname >/dev/null 2>&1; then
    uname_out="$(uname)"
    if [[ "${uname_out}" == "" ]]; then
      return 1
    else
      echo "${uname_out}"
      return 0
    fi
  else
    return 20
  fi
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  determine_arch
#   DESCRIPTION:  Attempt to determine architecture of host
#    PARAMETERS:  none
#       RETURNS:  0 = Arch Detected. Also prints detected arch to stdout
#                 1 = Unknown arch
#                 20 = 'uname' not found in path
#-------------------------------------------------------------------------------
determine_arch() {
  local uname_out

  if command -v uname >/dev/null 2>&1; then
    uname_out="$(uname -m)"
    if [[ "${uname_out}" == "" ]]; then
      return 1
    else
      echo "${uname_out}"
      return 0
    fi
  else
    return 20
  fi
}


#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  download_file
#   DESCRIPTION:  Downloads a file into the specified directory.  Attempts to
#                 use curl, then wget.  If neither is found, fail.
#    PARAMETERS:  $1 = url of file to download
#                 $2 = location to download file into on host system
#                 $3 = filename to save as
#       RETURNS:  If curl or wget found, returns the return code of curl or wget
#                 20 = Could not find curl and wget
#-------------------------------------------------------------------------------
download_file() {
  local url
  local dir
  local filename
  local rcode

  url="${1}"
  dir="${2}"
  filename="${3}"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}" -o "${dir}/${filename}"
    rcode="${?}"
  elif command -v wget >/dev/null 2>&1; then
    wget --quiet  "${url}" -O "${dir}/${filename}"
    rcode="${?}"
  else
    rcode="20"
  fi
  
  return "${rcode}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  checksum_check
#   DESCRIPTION:  Attempt to verify checksum of downloaded file to ensure
#                 integrity.  Tries multiple tools before failing.
#    PARAMETERS:  $1 = path to checksum file
#                 $2 = location of file to check
#                 $3 = working directory
#       RETURNS:  0 = checksum verified
#                 1 = checksum verification failed
#                 20 = failed to determine tool to use to check checksum
#                 30 = failed to change into or go back from working dir
#-------------------------------------------------------------------------------
checksum_check() {
  local checksum_file
  local file
  local dir
  local rcode
  local shasum_1
  local shasum_2
  local shasum_c

  checksum_file="${1}"
  file="${2}"
  dir="${3}"

  cd "${dir}" || return 30
  if command -v sha256sum >/dev/null 2>&1; then
    ## Not all sha256sum versions seem to have --ignore-missing, so filter the checksum file
    ## to only include the file we downloaded.
    grep "$(basename "${file}")" "${checksum_file}" > filtered_checksum.txt
    shasum_c="$(sha256sum -c "filtered_checksum.txt")"
    rcode="${?}"
  elif command -v shasum >/dev/null 2>&1; then
    ## With shasum on FreeBSD, we don't get to --ignore-missing, so filter the checksum file
    ## to only include the file we downloaded.
    grep "$(basename "${file}")" "${checksum_file}" > filtered_checksum.txt
    shasum_c="$(shasum -a 256 -c "filtered_checksum.txt")"
    rcode="${?}"
  elif command -v sha256 >/dev/null 2>&1; then
    ## With sha256 on FreeBSD, we don't get to --ignore-missing, so filter the checksum file
    ## to only include the file we downloaded.
    ## Also sha256 -c option seems to fail, so fall back to an if statement
    grep "$(basename "${file}")" "${checksum_file}" > filtered_checksum.txt
    shasum_1="$(sha256 -q "${file}")"
    shasum_2="$(awk '{print $1}' filtered_checksum.txt)"
    if [[ "${shasum_1}" == "${shasum_2}" ]]; then
      rcode="0"
    else
      rcode="1"
    fi
    shasum_c="Expected: ${shasum_1}, Got: ${shasum_2}"
  else
    return 20
  fi
  cd - >/dev/null 2>&1 || return 30
  
  if [[ "${rcode}" -gt "0" ]]; then
    echo "${shasum_c}"
  fi
  return "${rcode}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  extract_file
#   DESCRIPTION:  Extracts a file into a location.  Attempts to determine which
#                 tool to use by checking file extension.
#    PARAMETERS:  $1 = file to extract
#                 $2 = location to extract file into
#                 $3 = extension
#       RETURNS:  Return code of the tool used to extract the file
#                 20 = Failed to determine which tool to use
#                 30 = Failed to find tool in path
#-------------------------------------------------------------------------------
extract_file() {
  local file
  local dir
  local ext
  local rcode

  file="${1}"
  dir="${2}"
  ext="${3}"

  case "${ext}" in
       "zip" ) if command -v unzip >/dev/null 2>&1; then
                 unzip "${file}" -d "${dir}"
                 rcode="${?}"
               else
                 rcode="30"
               fi
               ;;
    "tar.gz" ) if command -v tar >/dev/null 2>&1; then
                 tar -xf "${file}" -C "${dir}"
                 rcode="${?}"
               else
                 rcode="31"
               fi
               ;;
           * ) rcode="20";;
  esac

  return "${rcode}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  create_prefix
#   DESCRIPTION:  Creates the install prefix (and any parent directories). If
#                 EUID not 0, then attempt to use sudo.
#    PARAMETERS:  $1 = prefix
#       RETURNS:  Return code of the tool used to make the directory
#                 0 = Created the directory
#                 >0 = Failed to create directory  
#                 20 = Could not find mkdir command
#                 21 = Could not find sudo command
#-------------------------------------------------------------------------------
create_prefix() {
  local prefix
  local rcode

  prefix="${1}"

  if command -v mkdir >/dev/null 2>&1; then
    if [[ "${EUID}" == "0" ]]; then
      mkdir -p "${prefix}"
      rcode="${?}"
    else
      if command -v sudo >/dev/null 2>&1; then
        sudo mkdir -p "${prefix}"
        rcode="${?}"
      else
        rcode="21"
      fi
    fi
  else
    rcode="20"
  fi

  return "${rcode}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  install_file_freebsd
#   DESCRIPTION:  Installs a file into a location using 'install'.  If EUID not
#                 0, then attempt to use sudo.
#    PARAMETERS:  $1 = file to install
#                 $2 = location to install file into
#       RETURNS:  0 = File Installed
#                 1 = File not installed
#                 20 = Could not find install command
#                 21 = Could not find sudo command
#-------------------------------------------------------------------------------
install_file_freebsd() {
  local file
  local prefix
  local rcode

  file="${1}"
  prefix="${2}"

  if command -v install >/dev/null 2>&1; then
    if [[ "${EUID}" == "0" ]]; then
      install -C -b -B '_old' -m 755 "${file}" "${prefix}"
      rcode="${?}"
    else
      if command -v sudo >/dev/null 2>&1; then
        sudo install -C -b -B '_old' -m 755 "${file}" "${prefix}"
        rcode="${?}"
      else
        rcode="21"
      fi
    fi
  else
    rcode="20"
  fi

  return "${rcode}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  install_file_linux
#   DESCRIPTION:  Installs a file into a location using 'install'.  If EUID not
#                 0, then attempt to use sudo (unless on android).
#    PARAMETERS:  $1 = file to install
#                 $2 = location to install file into
#       RETURNS:  0 = File Installed
#                 1 = File not installed
#                 20 = Could not find install command
#                 21 = Could not find sudo command
#-------------------------------------------------------------------------------
install_file_linux() {
  local file
  local prefix
  local rcode

  file="${1}"
  prefix="${2}"

  if command -v install >/dev/null 2>&1; then
    if [[ "${EUID}" == "0" ]]; then
      install -C -b -S '_old' -m 755 -t "${prefix}" "${file}"
      rcode="${?}"
    else
      if command -v sudo >/dev/null 2>&1; then
        sudo install -C -b -S '_old' -m 755 "${file}" "${prefix}"
        rcode="${?}"
      elif [[ "${ANDROID_ROOT}" != "" ]]; then
        install -C -b -S '_old' -m 755 -t "${prefix}" "${file}"
        rcode="${?}"
      else
        rcode="21"
      fi
    fi
  else
    rcode="20"
  fi

  return "${rcode}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  install_file_cygwin
#   DESCRIPTION:  Installs a file into a location using 'install'.  If EUID not
#                 0, then attempt to use sudo.
#                 Not really 100% sure this is how to install warpdrop in cygwin.
#    PARAMETERS:  $1 = file to install
#                 $2 = location to install file into
#       RETURNS:  0 = File Installed
#                 20 = Could not find install command
#                 21 = Could not find sudo command
#-------------------------------------------------------------------------------
install_file_cygwin() {
  local file
  local prefix
  local rcode

  file="${1}"
  prefix="${2}"

  if command -v install >/dev/null 2>&1; then
    if [[ "${EUID}" == "0" ]]; then
      install -m 755 "${prefix}" "${file}"
      rcode="${?}"
    else
      if command -v sudo >/dev/null 2>&1; then
        sudo install -m 755 "${file}" "${prefix}"
        rcode="${?}"
      else
        rcode="21"
      fi
    fi
  else
    rcode="20"
  fi

  return "${rcode}"
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  get_latest_version
#   DESCRIPTION:  Fetch the latest release version from GitHub API
#       RETURNS:  0 = Version fetched. Also prints version to stdout
#                 1 = Failed to fetch version
#                 20 = curl/wget not found
#-------------------------------------------------------------------------------
get_latest_version() {
  local api_url="https://api.github.com/repos/BioHazard786/Warpdrop/releases/latest"
  local response
  local version

  # 1. Fetch the response content first
  if command -v curl >/dev/null 2>&1; then
    response="$(curl -fsSL "${api_url}" 2>/dev/null)"
  elif command -v wget >/dev/null 2>&1; then
    response="$(wget -qO- "${api_url}" 2>/dev/null)"
  else
    return 20 # Missing dependencies
  fi

  # 2. Parse with robust regex (handles optional spaces)
  # Logic: Find "tag_name", allow optional spaces/colon, capture content inside quotes
  version="$(echo "${response}" | grep -o '"tag_name":[[:space:]]*"[^"]*"' | cut -d'"' -f4 | sed 's/^v//')"

  if [[ -z "${version}" ]]; then
    return 1 # Failed to find version
  fi

  echo "${version}"
  return 0
}

#---  FUNCTION  ----------------------------------------------------------------
#          NAME:  main
#   DESCRIPTION:  Put it all together in a logical way
#                 ...at least that is the hope...
#    PARAMETERS:  1 = prefix
#       RETURNS:  0 = All good
#                 1 = Something done broke
#-------------------------------------------------------------------------------
main() {
  local prefix
  local tmpdir
  local tmpdir_rcode
  local warpdrop_arch
  local warpdrop_arch_rcode
  local warpdrop_os
  local warpdrop_os_rcode
  local warpdrop_base_url
  local warpdrop_url
  local warpdrop_file
  local warpdrop_checksum_file
  local warpdrop_bin_name
  local warpdrop_version
  local warpdrop_dl_ext
  local download_file_rcode
  local download_checksum_file_rcode
  local checksum_check_rcode
  local extract_file_rcode
  local install_file_rcode
  local create_prefix_rcode

  warpdrop_bin_name="warpdrop"
  warpdrop_dl_ext="tar.gz"
  warpdrop_base_url="https://github.com/BioHazard786/WarpDrop/releases/download"
  prefix="${1}"

  print_banner
  print_message "== Fetching latest WarpDrop version..." "info"
  warpdrop_version="$(get_latest_version)"
  if [[ $? -eq 0 ]]; then
    print_message "== Latest version detected: ${warpdrop_version}" "ok"
  else
    print_message "== Failed to fetch latest version from GitHub, falling back to v0.0.2" "warn"
    warpdrop_version="0.0.2"
  fi
  
  print_message "== Install prefix set to ${prefix}" "info"
  
  tmpdir="$(make_tempdir "${warpdrop_bin_name}")"
  tmpdir_rcode="${?}"
  if [[ "${tmpdir_rcode}" == "0" ]]; then
    print_message "== Created temp dir at ${tmpdir}" "info"
  elif [[ "${tmpdir_rcode}" == "1" ]]; then
    print_message "== Failed to create temp dir at ${tmpdir}" "error"
  else
    print_message "== 'mktemp' not found in path. Is it installed?" "error"
    exit 1
  fi

  warpdrop_arch="$(determine_arch)"
  warpdrop_arch_rcode="${?}"
  if [[ "${warpdrop_arch_rcode}" == "0" ]]; then
    print_message "== Architecture detected as ${warpdrop_arch}" "info"
  elif [[ "${warpdrop_arch_rcode}" == "1" ]]; then
    print_message "== Architecture not detected" "error"
    exit 1
  else
    print_message "== 'uname' not found in path. Is it installed?" "error"
    exit 1
  fi

  warpdrop_os="$(determine_os)"
  warpdrop_os_rcode="${?}"
  if [[ "${warpdrop_os_rcode}" == "0" ]]; then
    print_message "== OS detected as ${warpdrop_os}" "info"
  elif [[ "${warpdrop_os_rcode}" == "1" ]]; then
    print_message "== OS not detected" "error"
    exit 1
  else
    print_message "== 'uname' not found in path. Is it installed?" "error"
    exit 1
  fi

  case "${warpdrop_os}" in
     "Darwin" ) warpdrop_os="macOS";;
    *"BusyBox"* )
        warpdrop_os="Linux"
        ;;
    "MINGW"* ) warpdrop_os="Windows";
                warpdrop_dl_ext="zip";;
    "CYGWIN"* ) warpdrop_os="Windows";
                warpdrop_dl_ext="zip";
                print_message "== Cygwin is currently unsupported." "error";
                exit 1;;
  esac

  case "${warpdrop_arch}" in
     "x86_64" ) warpdrop_arch="64bit";;
      "amd64" ) warpdrop_arch="64bit";;
    "aarch64" ) warpdrop_arch="ARM64";;
      "arm64" ) warpdrop_arch="ARM64";;
     "armv7l" ) warpdrop_arch="ARM";;
     "armv8l" ) warpdrop_arch="ARM";;
     "armv9l" ) warpdrop_arch="ARM";;
       "i686" ) warpdrop_arch="32bit";;
    "riscv64" ) warpdrop_arch="RISCV64";;
            * ) warpdrop_arch="unknown";;
  esac

  warpdrop_file="${warpdrop_bin_name}_v${warpdrop_version}_${warpdrop_os}-${warpdrop_arch}.${warpdrop_dl_ext}"
  warpdrop_checksum_file="${warpdrop_bin_name}_v${warpdrop_version}_checksums.txt"
  warpdrop_url="${warpdrop_base_url}/v${warpdrop_version}/${warpdrop_file}"
  warpdrop_checksum_url="${warpdrop_base_url}/v${warpdrop_version}/${warpdrop_checksum_file}"
  
  download_file "${warpdrop_url}" "${tmpdir}" "${warpdrop_file}"
  download_file_rcode="${?}"
  if [[ "${download_file_rcode}" == "0" ]]; then
    print_message "== Downloaded WarpDrop CLI archive into ${tmpdir}" "info"
  elif [[ "${download_file_rcode}" == "1" ]]; then
    print_message "== Failed to download WarpDrop CLI archive" "error"
    exit 1
  elif [[ "${download_file_rcode}" == "20" ]]; then
    print_message "== Failed to locate curl or wget" "error"
    exit 1
  else
    print_message "== Return code of download tool returned an unexpected value of ${download_file_rcode}" "error"
    exit 1
  fi

  download_file "${warpdrop_checksum_url}" "${tmpdir}" "${warpdrop_checksum_file}"
  download_checksum_file_rcode="${?}"
  if [[ "${download_checksum_file_rcode}" == "0" ]]; then
    print_message "== Downloaded WarpDrop CLI checksums file into ${tmpdir}" "info"
  elif [[ "${download_checksum_file_rcode}" == "1" ]]; then
    print_message "== Failed to download WarpDrop CLI checksums" "error"
    exit 1
  elif [[ "${download_checksum_file_rcode}" == "20" ]]; then
    print_message "== Failed to locate curl or wget" "error"
    exit 1
  else
    print_message "== Return code of download tool returned an unexpected value of ${download_checksum_file_rcode}" "error"
    exit 1
  fi

  checksum_check "${tmpdir}/${warpdrop_checksum_file}" "${tmpdir}/${warpdrop_file}" "${tmpdir}"
  checksum_check_rcode="${?}"
  if [[ "${checksum_check_rcode}" == "0" ]]; then
    print_message "== Checksum of ${tmpdir}/${warpdrop_file} verified" "ok"
  elif [[ "${checksum_check_rcode}" == "1" ]]; then
    print_message "== Failed to verify checksum of ${tmpdir}/${warpdrop_file}" "error"
    exit 1
  elif [[ "${checksum_check_rcode}" == "20" ]]; then
    print_message "== Failed to find tool to verify sha256 sums" "error"
    exit 1
  elif [[ "${checksum_check_rcode}" == "30" ]]; then
    print_message "== Failed to change into working directory ${tmpdir}" "error"
    exit 1
  else
    print_message "== Unknown return code returned while checking checksum of ${tmpdir}/${warpdrop_file}. Returned ${checksum_check_rcode}" "error"
    exit 1
  fi

  extract_file "${tmpdir}/${warpdrop_file}" "${tmpdir}/" "${warpdrop_dl_ext}"
  extract_file_rcode="${?}"
  if [[ "${extract_file_rcode}" == "0" ]]; then
    print_message "== Extracted ${warpdrop_file} to ${tmpdir}/" "info"
  elif [[ "${extract_file_rcode}" == "1" ]]; then
    print_message "== Failed to extract ${warpdrop_file}" "error"
    exit 1
  elif [[ "${extract_file_rcode}" == "20" ]]; then
    print_message "== Failed to determine which extraction tool to use" "error"
    exit 1
  elif [[ "${extract_file_rcode}" == "30" ]]; then
    print_message "== Failed to find 'unzip' in path" "error"
    exit 1
  elif [[ "${extract_file_rcode}" == "31" ]]; then
    print_message "== Failed to find 'tar' in path" "error"
    exit 1
  else
    print_message "== Unknown error returned from extraction attempt" "error"
    exit 1
  fi

  if [[ ! -d "${prefix}" ]]; then
    create_prefix "${prefix}"
    create_prefix_rcode="${?}"
    if [[ "${create_prefix_rcode}" == "0" ]]; then
      print_message "== Created install prefix at ${prefix}" "info"
    elif [[ "${create_prefix_rcode}" == "20" ]]; then
      print_message "== Failed to find mkdir in path" "error"
      exit 1
    elif [[ "${create_prefix_rcode}" == "21" ]]; then
      print_message "== Failed to find sudo in path" "error"
      exit 1
    else
      print_message "== Failed to create the install prefix: ${prefix}" "error"
      exit 1
    fi
  else
    print_message "== Install prefix already exists. No need to create it." "info"
  fi

  case "${warpdrop_os}" in
    "Linux" ) install_file_linux "${tmpdir}/${warpdrop_bin_name}" "${prefix}/";
              install_file_rcode="${?}";;
  "FreeBSD" ) install_file_freebsd "${tmpdir}/${warpdrop_bin_name}" "${prefix}/";
              install_file_rcode="${?}";;
    "macOS" ) install_file_freebsd "${tmpdir}/${warpdrop_bin_name}" "${prefix}/";
              install_file_rcode="${?}";;
  "Windows" ) install_file_cygwin "${tmpdir}/${warpdrop_bin_name}" "${prefix}/";
              install_file_rcode="${?}";;
  esac

  if [[ "${install_file_rcode}" == "0" ]] ; then
    print_message "== Installed ${warpdrop_bin_name} to ${prefix}/" "ok"
  elif [[ "${install_file_rcode}" == "1" ]]; then
    print_message "== Failed to install ${warpdrop_bin_name}" "error"
    exit 1
  elif [[ "${install_file_rcode}" == "20" ]]; then
    print_message "== Failed to locate 'install' command" "error"
    exit 1
  elif [[ "${install_file_rcode}" == "21" ]]; then
    print_message "== Failed to locate 'sudo' command" "error"
    exit 1
  else
    print_message "== Install attempt returned an unexpected value of ${install_file_rcode}" "error"
    exit 1
  fi

  print_message "== Installation complete" "ok"
  
  exit 0
}

#-------------------------------------------------------------------------------
#  ARGUMENT PARSING
#-------------------------------------------------------------------------------
OPTS="hp:"
while getopts "${OPTS}" optchar; do
  case "${optchar}" in
    'h' ) print_help
          exit 0
          ;;
    'p' ) INSTALL_PREFIX="${OPTARG}"
          ;;
     /? ) print_message "Unknown option ${OPTARG}" "warn"
          ;;
  esac
done

#-------------------------------------------------------------------------------
# CALL MAIN
#-------------------------------------------------------------------------------
main "${INSTALL_PREFIX}"
