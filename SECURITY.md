# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of Bud seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please Do Not

- Open a public GitHub issue for security vulnerabilities
- Disclose the vulnerability publicly before it has been addressed

### Please Do

**Report security vulnerabilities via GitHub Security Advisories:**

1. Go to the [Security tab](https://github.com/mskutin/Bud/security) of the repository
2. Click "Report a vulnerability"
3. Fill out the form with details about the vulnerability

**Or email us directly:**

Send details to: security@mskutin.com (replace with actual email)

### What to Include

Please include the following information in your report:

- Type of vulnerability (e.g., authentication bypass, injection, etc.)
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability, including how an attacker might exploit it

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours
- **Updates**: We will send you regular updates about our progress
- **Timeline**: We aim to address critical vulnerabilities within 7 days
- **Credit**: We will credit you in the security advisory (unless you prefer to remain anonymous)

## Security Best Practices

When using Bud:

### AWS Credentials

- **Never commit AWS credentials** to version control
- Use IAM roles with least-privilege permissions
- Rotate credentials regularly
- Use AWS SSO or temporary credentials when possible

### Configuration Files

- Keep `.Bud.yaml` out of version control (it's in .gitignore)
- Use environment variables for sensitive configuration
- Review configuration files before sharing

### Cross-Account Access

- Use dedicated IAM roles for cross-account access
- Implement role assumption with external ID for additional security
- Regularly audit role permissions

### Running the Tool

- Run with minimum required AWS permissions
- Use read-only permissions where possible
- Review recommendations before applying changes
- Test in non-production environments first

## Known Security Considerations

### AWS API Access

This tool requires:
- Read access to AWS Cost Explorer API
- Read access to AWS Budgets API
- Read access to AWS Organizations API (for multi-account)
- Optional: AssumeRole permissions for cross-account access

### Data Handling

- Cost data is processed in-memory only
- No data is sent to external services
- JSON exports contain cost information - handle appropriately
- Logs may contain account IDs - review before sharing

### Dependencies

We use Dependabot to monitor dependencies for known vulnerabilities. Security updates are applied promptly.

## Security Updates

Security updates will be released as patch versions (e.g., 1.0.1) and announced via:

- GitHub Security Advisories
- GitHub Releases
- Repository README

Subscribe to repository notifications to stay informed about security updates.

## Disclosure Policy

When we receive a security bug report, we will:

1. Confirm the problem and determine affected versions
2. Audit code to find similar problems
3. Prepare fixes for all supported versions
4. Release new versions as soon as possible
5. Publish a security advisory on GitHub

## Comments on This Policy

If you have suggestions on how this process could be improved, please submit a pull request or open an issue.
