# Kind E2E Test Structure

## 🎯 Overview

The Kind E2E tests follow the same pattern as other E2E tests: **setup script + Go tests**. This ensures consistency and maintainability across all E2E test environments.

## 📁 File Structure

```
e2e/
├── kind_setup.sh          # Setup script (creates cluster, deploys services)
├── kind_test.go           # Go test file (verifies functionality)
├── run_kind_tests.sh      # Test runner (setup + run tests)
└── KIND_E2E_TEST_STRUCTURE.md  # This documentation
```

## 🚀 Usage

### Run All Kind E2E Tests:
```bash
cd e2e
./run_kind_tests.sh
```

### Run Setup Only:
```bash
cd e2e
./kind_setup.sh
```

### Run Tests Only (after setup):
```bash
cd e2e
go test -v -timeout=5m ./e2e -run "TestKind"
```

## 🧪 Test Scenarios

### 1. **Local Priority Tests** (`TestKindLocalPriority`)
**Verifies that packages available in both indexes are served from local PyPI (priority)**

Packages tested:
- `flask`, `click`, `jinja2`, `werkzeug`
- `six`, `itsdangerous`, `blinker`, `requests`

**Key Assertions:**
- ✅ Package metadata served from local PyPI
- ✅ Package URLs show local server paths (`/packages/`)
- ✅ No public PyPI URLs in metadata
- ✅ `X-PyPI-Source` header shows local server

### 2. **Public Only Tests** (`TestKindPublicOnly`)
**Verifies that packages only available in public PyPI are served from public PyPI**

Packages tested:
- `numpy`, `pandas`, `matplotlib`, `scipy`
- `urllib3`, `certifi`

**Key Assertions:**
- ✅ Package metadata served from public PyPI
- ✅ Package URLs show public PyPI paths (`files.pythonhosted.org`)
- ✅ `X-PyPI-Source` header shows public PyPI

### 3. **File Download Tests** (`TestKindFileDownload`)
**Verifies that package files are downloaded from the correct source**

**Local packages (from local PyPI):**
- `flask-2.3.3.tar.gz`, `click-8.1.7.tar.gz`, `requests-2.31.0.tar.gz`

**Public packages (from public PyPI):**
- `numpy-1.24.3.tar.gz`, `pandas-2.0.3.tar.gz`

### 4. **Health and Index Tests**
- `TestKindProxyHealth`: Verifies proxy health endpoint
- `TestKindProxyIndex`: Verifies proxy index page
- `TestKindLocalServerAccess`: Verifies direct local server access

## 🔍 Enhanced Logging Verification

The tests verify that Tejedor's enhanced logging shows correct routing decisions:

**Expected Log Patterns:**
```
ROUTING: /simple/flask/ - publicExists=false, privateExists=true
ROUTING: /simple/flask/ → LOCAL_PYPI (http://test-pypi-server-service:8080/simple/)
FILE: /packages/flask-2.3.3.tar.gz → LOCAL_PYPI (http://test-pypi-server-service:8080/simple/)

ROUTING: /simple/numpy/ - publicExists=true, privateExists=false
ROUTING: /simple/numpy/ → PUBLIC_PYPI (https://pypi.org/simple/)
FILE: /packages/numpy-1.24.3.tar.gz → PUBLIC_PYPI (https://pypi.org/simple/)
```

## 📦 Package Population

The setup script populates the local PyPI server with packages from `populate-requirements.txt`:

**Packages in Local PyPI (served with priority):**
- `flask==2.3.3`, `click==8.1.7`, `jinja2==3.1.2`
- `werkzeug==2.3.7`, `six==1.16.0`, `itsdangerous==2.1.2`
- `blinker==1.6.3`, `requests==2.31.0`

**Packages in Public PyPI only (served as fallback):**
- `numpy`, `pandas`, `matplotlib`, `scipy`
- `urllib3`, `certifi`, and many others

## 🎯 Key Verifications

### **Priority Routing Verification:**
1. **Packages in both indexes** → Served from **local PyPI** (priority)
2. **Packages in public only** → Served from **public PyPI** (fallback)
3. **File downloads** → Routed to correct source
4. **Enhanced logging** → Shows routing decisions clearly

### **Evidence Collection:**
- **HTTP Headers**: `X-PyPI-Source` shows which backend served the request
- **Package Metadata**: URLs in metadata show the source
- **Enhanced Logs**: Direct evidence of routing decisions
- **File Content**: Different file sizes/content for local vs public

## 🔧 Environment Setup

The setup script creates:
1. **Kind cluster** with port mappings
2. **Tejedor proxy** with enhanced logging
3. **Local PyPI server** populated with test packages
4. **Port forwarding** for external access
5. **Connectivity verification** for all services

## 🧹 Cleanup

The test runner automatically:
- Cleans up port forwarding processes
- Deletes the Kind cluster (via trap in setup script)
- Shows final test results and logs

## 📊 Test Results

**Success Criteria:**
- ✅ All Go tests pass
- ✅ Enhanced logging shows correct routing
- ✅ Packages served from correct backends
- ✅ File downloads work correctly
- ✅ Health endpoints respond correctly

**Failure Investigation:**
- Check Tejedor logs for routing decisions
- Verify local PyPI server population
- Confirm port forwarding is working
- Check package availability in both indexes 