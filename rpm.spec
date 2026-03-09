# Adapted from the Fedora Rawhide ollama.spec for building directly from a
# local git checkout.  No source tarballs, vendor tarballs, or SRPMs are used.
#
# Usage (from the repository root):
#   rpmbuild -bb rpm.spec [--define "sourcetree $(pwd)"]
#
# sourcetree defaults to the directory that contains this spec file so the
# above --define is normally not required when invoking rpmbuild with the full
# path to the spec:
#   rpmbuild -bb /src/ollama/rpm.spec
#
# The build requires network access during %%prep to populate the vendor/
# directory via "go mod vendor".

%bcond check 0
%ifarch x86_64
%bcond rocm 1
%else
%bcond rocm 0
%endif
%bcond vulkan 1

# bundled GGML sources are missing ppc64le and s390x
ExcludeArch:    ppc64le s390x

# Build the next version of ollama
%bcond next 0

# systemd integration
%bcond systemd 1

# ---------------------------------------------------------------------------
# Go import path and version
# ---------------------------------------------------------------------------
%global goipath  github.com/ollama/ollama
%global gourl    https://github.com/ollama/ollama
Version:         0.17.7

# Location of the git checkout.  Overridable at rpmbuild invocation time:
#   rpmbuild -bb rpm.spec --define "sourcetree /path/to/ollama"
# The default probes a short candidate list for the first directory that
# contains go.mod, so "rpmbuild -bb /src/ollama/rpm.spec" just works.
# (%%{_specdir} always points to ~/rpmbuild/SPECS, not the spec file's real
# location, so we cannot rely on it alone.)
%{!?sourcetree:%global sourcetree %(for d in "%{_specdir}" /src/ollama "$(pwd)"; do [ -f "$d/go.mod" ] && echo "$d" && break; done)}

# gobuilddir must be defined before %%gobuild is called.  The standard
# golang-rpm-macros expand %%gobuilddir as %%{_builddir}/%%{buildsubdir}/_build
# where buildsubdir is set by %%setup.  We mirror that here explicitly so the
# macro resolves even though we replace %%setup with a manual copy.
%global gobuilddir %{_builddir}/%{name}-%{version}/_build

# ---------------------------------------------------------------------------
Name:           ollama
Release:        %{?releasever}%{!?releasever:1}%{?dist}
Summary:        Get up and running with OpenAI gpt-oss, DeepSeek-R1, Gemma 3 and other models

# go-vendor-tools license expression (kept verbatim from upstream Fedora spec)
License:        Apache-2.0 AND BSD-2-Clause AND BSD-3-Clause AND BSD-3-Clause-HP AND BSL-1.0 AND CC-BY-3.0 AND CC-BY-4.0 AND CC0-1.0 AND ISC AND LicenseRef-Fedora-Public-Domain AND LicenseRef-scancode-protobuf AND MIT AND NCSA AND NTP AND OpenSSL AND ZPL-2.1 AND Zlib
URL:            %{gourl}

# Fedora-specific patch: register /load without the method-qualified prefix so
# that old clients using plain GET/POST still work.


BuildRequires:  golang
BuildRequires:  fdupes
BuildRequires:  gcc-c++
BuildRequires:  cmake

%if %{with systemd}
BuildRequires:  systemd-rpm-macros
%endif

%if %{with rocm}
BuildRequires:  hipblas-devel
BuildRequires:  rocblas-devel
BuildRequires:  rocm-comgr-devel
BuildRequires:  rocm-compilersupport-macros
BuildRequires:  rocm-runtime-devel
BuildRequires:  rocm-hip-devel
BuildRequires:  rocm-rpm-macros
BuildRequires:  rocminfo
%endif

%if %{with vulkan}
BuildRequires:  vulkan-loader-devel
BuildRequires:  glslc
%endif

Requires:       %{name}-base%{?_isa} = %{version}-%{release}
%if %{with rocm}
Requires:       %{name}-rocm%{?_isa} = %{version}-%{release}
%endif
%if %{with vulkan}
Requires:       %{name}-vulkan%{?_isa} = %{version}-%{release}
%endif

%description
Get up and running with OpenAI gpt-oss, DeepSeek-R1, Gemma 3 and other models.

%package base
Summary:        The base ollama
%if %{with systemd}
%{?systemd_requires}
%endif

%description base
%{summary}

%if %{with rocm}
%package rocm
Summary:        The ROCm backend for ollama
Requires:       hipblas
Requires:       rocblas

%description rocm
%{summary}
%endif

%if %{with vulkan}
%package vulkan
Summary:        The Vulkan backend for ollama

%description vulkan
%{summary}
%endif


# ---------------------------------------------------------------------------
%prep
# Create the build directory without unpacking any tarball (-T), then populate
# it by copying the local git checkout.
%setup -c -T -n %{name}-%{version}
cp -rp %{sourcetree}/. .

# Rename READMEs that would collide with the top-level one
mv app/README.md app-README.md
mv integration/README.md integration-README.md
mv llama/README.md llama-README.md

# Disable Vulkan in cmake if the bcond is off
%if %{without vulkan}
sed -i -e 's@Vulkan_FOUND@FALSE@' CMakeLists.txt
%endif

# Populate the vendor directory from the module cache / network so that the
# Go build can use -mod=vendor (no pre-built vendor tarball is required).
go mod vendor


# ---------------------------------------------------------------------------
%build

%cmake \
%if %{with rocm}
    -DCMAKE_HIP_COMPILER=%rocmllvm_bindir/clang++ \
    -DAMDGPU_TARGETS=%{rocm_gpu_list_default}
%endif

%cmake_build

%global gomodulesmode GO111MODULE=on

# cmake sets LDFLAGS which confuses gobuild; reset it.
export LDFLAGS=
%gobuild -o %{gobuilddir}/bin/ollama %{goipath}


# ---------------------------------------------------------------------------
%install

%cmake_install

# Remove bundled copies of system libraries (ROCm / Vulkan / misc)
runtime_removal="hipblas rocblas amdhip64 rocsolver amd_comgr hsa-runtime64 rocsparse tinfo rocprofiler-register drm drm_amdgpu numa elf vulkan"
for rr in $runtime_removal; do
    rm -rf %{buildroot}%{_prefix}/lib/ollama/lib${rr}*
done
rm -rf %{buildroot}%{_prefix}/lib/ollama/rocblas

install -m 0755 -vd %{buildroot}%{_bindir}
install -m 0755 -vp %{gobuilddir}/bin/ollama %{buildroot}%{_bindir}/ollama

rm -rf %{buildroot}%{_bindir}/*.so

%if %{with systemd}
install -p -D -m 0644 ollama.service  %{buildroot}%{_unitdir}/ollama.service
install -p -D -m 0644 ollama.sysusers %{buildroot}%{_sysusersdir}/ollama.conf
mkdir -p %{buildroot}%{_var}/lib/ollama
%endif


# ---------------------------------------------------------------------------
%check
%if %{with check}
%gotest ./...
%endif


# ---------------------------------------------------------------------------
%if %{with systemd}
%preun
%systemd_preun ollama.service

%post
%systemd_post ollama.service

%postun
%systemd_postun_with_restart ollama.service
%endif


# ---------------------------------------------------------------------------
%files
%doc README.md


%files base
%license LICENSE vendor/modules.txt
%doc CONTRIBUTING.md SECURITY.md README.md app-README.md integration-README.md
%doc llama-README.md
%{_prefix}/lib/ollama/libggml-base.so
%{_prefix}/lib/ollama/libggml-base.so.0{,.*}
%ifarch x86_64
%{_prefix}/lib/ollama/libggml-cpu-alderlake.so
%{_prefix}/lib/ollama/libggml-cpu-haswell.so
%{_prefix}/lib/ollama/libggml-cpu-icelake.so
%{_prefix}/lib/ollama/libggml-cpu-sandybridge.so
%{_prefix}/lib/ollama/libggml-cpu-skylakex.so
%{_prefix}/lib/ollama/libggml-cpu-sse42.so
%{_prefix}/lib/ollama/libggml-cpu-x64.so
%else
# upstream CMakeLists.txt disables GGML CPU variants on aarch64
%{_prefix}/lib/ollama/libggml-cpu.so
%endif
%{_bindir}/ollama

%if %{with systemd}
%attr(0755,ollama,ollama) %dir %{_var}/lib/ollama/
%{_unitdir}/ollama.service
%{_sysusersdir}/ollama.conf
%endif


%if %{with rocm}
%files rocm
%{_prefix}/lib/ollama/libggml-hip.so
%endif

%if %{with vulkan}
%files vulkan
%{_prefix}/lib/ollama/libggml-vulkan.so
%endif


%changelog
%autochangelog
