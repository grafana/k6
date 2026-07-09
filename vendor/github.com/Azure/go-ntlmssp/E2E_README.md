# E2E NTLM Tests

This directory contains end-to-end tests for the go-ntlmssp library that test against real NTLM servers.

## Running E2E Tests Locally

### Prerequisites

- Windows machine with IIS capabilities
- Go 1.20 or later
- Administrator privileges (for IIS setup)

### Setup

1. **Enable IIS with Windows Authentication:**
   ```powershell
   # Run as Administrator
   Enable-WindowsOptionalFeature -Online -FeatureName IIS-WebServerRole -All
   Enable-WindowsOptionalFeature -Online -FeatureName IIS-WindowsAuthentication -All
   ```

2. **Create test site:**
   ```powershell
   Import-Module WebAdministration
   New-Website -Name "ntlmtest" -Port 8080 -PhysicalPath "C:\inetpub\wwwroot"
   Set-WebConfigurationProperty -Filter "/system.webServer/security/authentication/anonymousAuthentication" -Name enabled -Value false -PSPath "IIS:\Sites\ntlmtest"
   Set-WebConfigurationProperty -Filter "/system.webServer/security/authentication/windowsAuthentication" -Name enabled -Value true -PSPath "IIS:\Sites\ntlmtest"
   ```

3. **Set environment variables:**
   ```powershell
   $env:NTLM_TEST_URL = "http://localhost:8080/"
   $env:NTLM_TEST_USER = "your_username"
   $env:NTLM_TEST_PASSWORD = "your_password" 
   $env:NTLM_TEST_DOMAIN = "your_domain"  # Optional
   ```
   
   > **Note**: The setup script automatically generates a random secure password if none is provided. For security, avoid hardcoded passwords in scripts or CI environments.

4. **Run tests:**
   ```bash
   go test -v -tags=e2e ./e2e -run TestNTLM_E2E
   ```

## GitHub Actions

The E2E tests run automatically in GitHub Actions on Windows runners. The workflow:

1. Sets up a clean Windows Server environment
2. Generates a random secure password for the test user
3. Creates a test user account with the random password
4. Configures IIS with Windows Authentication
5. Runs the E2E tests against the real NTLM server
5. Cleans up resources

## Test Coverage

The E2E tests cover:

- ✅ Basic NTLM authentication flow
- ✅ UPN format usernames (`user@domain.com`)
- ✅ SAM format usernames (`DOMAIN\user`) 
- ✅ Authentication failure scenarios
- ✅ Server accessibility checks
- ✅ Context cancellation handling
- ✅ Direct ProcessChallenge function testing

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NTLM_TEST_URL` | URL of NTLM-enabled server | `http://localhost:8080/` |
| `NTLM_TEST_USER` | Username for authentication | `$USERNAME` (Windows) |
| `NTLM_TEST_PASSWORD` | Password for authentication | Required |
| `NTLM_TEST_DOMAIN` | Domain for authentication | `$USERDOMAIN` (Windows) |

## Troubleshooting

### Common Issues

1. **"No username available"** - Set `NTLM_TEST_USER` environment variable
2. **"No password available"** - Set `NTLM_TEST_PASSWORD` environment variable  
3. **Connection refused** - Ensure IIS is running and accessible on the specified port
4. **401 Unauthorized** - Check that Windows Authentication is enabled and working

### IIS Debugging

Check IIS status:
```powershell
Get-Website
Get-WebApplication
Get-WebConfigurationProperty -Filter "/system.webServer/security/authentication/windowsAuthentication" -Name enabled -PSPath "IIS:\Sites\Default Web Site"
```

View IIS logs:
```powershell
Get-Content "C:\inetpub\logs\LogFiles\W3SVC1\*.log" | Select-Object -Last 50
```

## Security Note

These tests use real authentication credentials. In CI/CD:
- Test credentials are generated dynamically per job
- Credentials are cleaned up after each test run
- No persistent credentials are stored

For local development, use test accounts or ensure credentials are not committed to version control.