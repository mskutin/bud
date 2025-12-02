# Bud

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![codecov](https://codecov.io/gh/mskutin/bud/branch/main/graph/badge.svg)](https://codecov.io/gh/mskutin/bud)

> Your AWS budget buddy

A command-line tool that analyzes AWS spending patterns and generates intelligent recommendations for [AWS Budgets](https://aws.amazon.com/aws-cost-management/aws-budgets/) configurations.

> **What is AWS Budgets?** AWS Budgets is an AWS service that lets you set custom cost and usage budgets that alert you when you exceed (or are forecasted to exceed) your budgeted amount. This tool helps you optimize those budget configurations based on actual spending patterns.

## 🚀 Quick Start

```bash
# Install (macOS/Linux)
brew install mskutin/tap/bud

# Install (Windows)
scoop bucket add mskutin https://github.com/mskutin/scoop-bucket
scoop install bud

# Or use Go
go install github.com/mskutin/bud@latest

# Run basic analysis
bud

# With cross-account budget access
bud --assume-role-name OrganizationAccountAccessRole

# With custom policies (see .bud.yaml.example)
bud --config .bud.yaml
```

**See [Installation](#installation) for all installation methods.**  
**See [Configuration](#configuration) for detailed setup options.**  
**See [ALTERNATIVES.md](ALTERNATIVES.md) for comparison with other tools.**

## What This Tool Does

This tool helps you optimize your **AWS Budgets** configurations by:

1. **Analyzing actual spending** from AWS Cost Explorer
2. **Comparing** against your configured AWS Budgets
3. **Recommending** optimal budget limits based on real usage patterns
4. **Identifying** accounts with misaligned budgets (too high or too low)

### Why Use This?

- **Prevent Alert Fatigue** - Stop getting budget alerts for accounts that consistently exceed poorly-configured limits
- **Catch Cost Overruns** - Ensure budgets are set high enough to catch real anomalies, not normal usage
- **Multi-Account Management** - Analyze dozens or hundreds of AWS accounts at once
- **Data-Driven Decisions** - Base budget limits on actual spending patterns, not guesswork

## Features

- 📊 **Automated Spend Analysis** - Retrieves historical cost data from AWS Cost Explorer API
- 💰 **Smart Recommendations** - Suggests AWS Budget limits based on peak spend + configurable growth buffer
- 🔍 **Multi-Account Support** - Discovers and analyzes all accounts in your AWS Organization
- 🔐 **Cross-Account Access** - Assumes roles in child accounts to read AWS Budgets configurations
- 📈 **Flexible Reporting** - Outputs recommendations in table or JSON format
- ⚡ **Concurrent Processing** - Fetches data from multiple accounts in parallel
- 🎯 **Priority Flagging** - Highlights accounts needing immediate attention

## How It Works

```
┌─────────────────┐
│ AWS Cost        │  1. Fetch historical spending data
│ Explorer API    │     (last 3-6 months)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ AWS Budgets     │  2. Retrieve current AWS Budget
│ API             │     configurations (if they exist)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Analysis        │  3. Calculate statistics:
│ Engine          │     - Average monthly spend
│                 │     - Peak monthly spend
│                 │     - Spending trends
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Recommendation  │  4. Generate AWS Budget recommendations:
│ Engine          │     - Peak spend × (1 + growth buffer)
│                 │     - Round to clean increments
│                 │     - Flag priority accounts
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Report          │  5. Output recommendations for
│                 │     updating your AWS Budgets
└─────────────────┘
```

## Installation

### macOS / Linux

**Homebrew:**
```bash
brew install mskutin/tap/bud
```

**Go Install:**
```bash
go install github.com/mskutin/bud@latest
```

### Windows

**Scoop:**
```powershell
scoop bucket add mskutin https://github.com/mskutin/scoop-bucket
scoop install bud
```

**Go Install:**
```powershell
go install github.com/mskutin/bud@latest
```

**Manual Download:**
Download the Windows binary from the [releases page](https://github.com/mskutin/bud/releases/latest), extract it, and add to your PATH.

### From Source

```bash
git clone https://github.com/mskutin/bud.git
cd bud
go build -o bud ./cmd/bud
```

## Quick Start

### Basic Usage

```bash
# Analyze all accounts and compare against AWS Budgets
./bud

# Access AWS Budgets in child accounts via role assumption
./bud --assume-role-name OrganizationAccountAccessRole

# Analyze only accounts in specific Organizational Units
./bud --organizational-units ou-xxxx-11111111,ou-yyyy-22222222

# Combine OU filtering with role assumption
./bud \
  --organizational-units ou-prod-12345678 \
  --assume-role-name OrganizationAccountAccessRole \
  --growth-buffer 15

# Export recommendations to JSON (shows table + saves JSON)
./bud --output-file recommendations.json

# Or explicitly specify JSON-only output
./bud --output-format json --output-file recommendations.json
```

## Configuration

### Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--analysis-months` | Number of months to analyze | 3 |
| `--growth-buffer` | Growth buffer percentage above peak | 20 |
| `--minimum-budget` | Minimum budget for any account (USD) | 10 |
| `--output-format` | Output format: table, json, or both | table |
| `--output-file` | File path for JSON export (auto-enables JSON) | - |
| `--assume-role-name` | Role name to assume in child accounts | - |
| `--aws-profile` | AWS profile to use | - |
| `--accounts` | Filter specific account IDs (comma-separated) | - |
| `--organizational-units` | Filter by OU IDs (comma-separated) | - |

### Output Formats

The tool supports three output formats:

- **`table`** (default) - Human-readable table to console
- **`json`** - JSON format (to console or file)
- **`both`** - Table to console + JSON to file

**Smart behavior:** When you specify `--output-file`, the tool automatically uses `both` format (shows table AND saves JSON), so you don't need to specify `--output-format both`.

```bash
# These are equivalent:
./bud --output-file budgets.json
./bud --output-format both --output-file budgets.json

# JSON only (no table):
./bud --output-format json --output-file budgets.json
```

### Configuration File

Create `.bud.yaml`:

```yaml
analysisMonths: 6
growthBuffer: 25
minimumBudget: 10
assumeRoleName: OrganizationAccountAccessRole

# Optional: Filter by Organizational Units
organizationalUnits:
  - "ou-prod-12345678"
  - "ou-staging-87654321"
```

## Per-OU/Account Policy Configuration

You can define different budget recommendation policies for different parts of your organization. This is useful when different teams, environments, or cost centers have different budget requirements.

### Policy Priority

Policies are resolved with the following priority (highest to lowest):

1. **Account Policy** - Specific to an individual account
2. **Tag Policy** - Based on account tags (e.g., Environment, CostCenter)
3. **OU Policy** - Applies to all accounts in an Organizational Unit
4. **Default Policy** - Global settings (top-level config values)

### Policy Inheritance

Policies inherit from the default configuration. You only need to specify values you want to override:

```yaml
# Global defaults
growthBuffer: 20
minimumBudget: 10
roundingIncrement: 10

# OU policy - only overrides growthBuffer and minimumBudget
ouPolicies:
  - ou: "ou-prod-12345678"
    name: "Production"
    growthBuffer: 15          # Override
    minimumBudget: 50         # Override
    # roundingIncrement: 10   # Inherited from default
```

### OU-Based Policies

Apply different policies to entire Organizational Units:

```yaml
ouPolicies:
  - ou: "ou-prod-12345678"
    name: "Production"
    growthBuffer: 15          # Conservative for production
    minimumBudget: 50         # Higher minimum
    roundingIncrement: 50     # Round to nearest $50

  - ou: "ou-dev-87654321"
    name: "Development"
    growthBuffer: 30          # More flexible for dev/test
    minimumBudget: 10

  - ou: "ou-sandbox-99999999"
    name: "Sandbox"
    growthBuffer: 50          # Very flexible for experimentation
    minimumBudget: 5
```

**Use Cases:**
- Production environments need tighter budget controls (lower growth buffer)
- Development/test environments can have more flexible budgets
- Sandbox accounts need minimal budgets

### Tag-Based Policies

Apply policies based on account tags:

```yaml
tagPolicies:
  - tagKey: "Environment"
    tagValue: "production"
    name: "Production (by tag)"
    growthBuffer: 15
    minimumBudget: 50

  - tagKey: "CostCenter"
    tagValue: "engineering"
    name: "Engineering"
    growthBuffer: 25
    roundingIncrement: 25

  - tagKey: "CostCenter"
    tagValue: "marketing"
    name: "Marketing"
    growthBuffer: 30
```

**Use Cases:**
- Tag accounts by environment (production, staging, development)
- Tag accounts by cost center or department
- Tag accounts by project or application

### Account-Specific Overrides

Highest priority - override policy for specific accounts:

```yaml
accountPolicies:
  - account: "123456789012"
    name: "Critical API"
    growthBuffer: 10          # Very tight control
    minimumBudget: 100
    roundingIncrement: 100

  - account: "234567890123"
    name: "Data Warehouse"
    growthBuffer: 5           # Predictable costs
    minimumBudget: 500
    roundingIncrement: 100
```

**Use Cases:**
- Critical production accounts need special attention
- High-cost accounts need different rounding increments
- Accounts with predictable costs need lower growth buffers

### Complete Policy Example

```yaml
# Global defaults
analysisMonths: 3
growthBuffer: 20
minimumBudget: 10
roundingIncrement: 10

# OU policies
ouPolicies:
  - ou: "ou-prod-12345678"
    name: "Production"
    growthBuffer: 15
    minimumBudget: 50
    roundingIncrement: 50

  - ou: "ou-dev-87654321"
    name: "Development"
    growthBuffer: 30
    minimumBudget: 10

# Tag policies
tagPolicies:
  - tagKey: "CostCenter"
    tagValue: "infrastructure"
    name: "Infrastructure"
    growthBuffer: 15
    minimumBudget: 200

# Account overrides
accountPolicies:
  - account: "999888777666"
    name: "Payment Processing"
    growthBuffer: 5
    minimumBudget: 1000
    roundingIncrement: 500
```

### Policy Transparency

The tool displays which policy was applied to each account in the report:

```
Priority  Account Name      Policy          Account ID      Current  Recommended  Adjustment
HIGH      Prod API          Production      123456789012    $500     $1070        +114.0%
HIGH      Dev Env           Development     234567890123    -        $80          NEW
MEDIUM    Critical API      Critical API    999888777666    $800     $1050        +31.3%
```

### Policy Validation

The tool validates your policy configuration on startup:

- **OU IDs** are checked to ensure they exist in your organization
- **Invalid OUs** cause the tool to fail fast with a clear error message
- **Policy conflicts** are resolved using the priority order

```bash
# If you specify an invalid OU
./bud

# Output:
# Validating 2 configured OU(s)...
# Error: policy configuration error: OU ou-invalid-12345678 does not exist or is not accessible
```

## Output Example

```
Priority  Account Name                         Account ID      Current     Average     Peak        Recommended   Adjustment
--------  -----------------------------------  --------------  ----------  ----------  ----------  ------------  ----------
HIGH      Production API                       123456789012          $500        $650        $890         $1070  +114.0%   
HIGH      Development Environment              234567890123             -         $45         $67           $80  NEW       
MEDIUM    Staging Environment                  345678901234          $200        $150        $180          $220  +10.0%    
```

### Adjustment Column

| Display | Meaning |
|---------|---------|
| **NEW** | No AWS Budget configured - needs to be created |
| **UNKNOWN** | AWS Budget may exist but access is denied |
| **+X%** | AWS Budget limit should increase by X% |
| **-X%** | AWS Budget limit can be reduced by X% |

### What to Do With These Recommendations

1. **Review** the recommendations in the report
2. **Update AWS Budgets** via:
   - AWS Console → Billing → Budgets
   - AWS CLI: `aws budgets update-budget`
   - Infrastructure as Code (Terraform, Pulumi, CloudFormation)
3. **Adjust notification thresholds** if needed (typically 90% for ACTUAL, 110% for FORECASTED)

## Cross-Account Setup

For organizations where **AWS Budgets are created in child accounts** (not the management account):

### 1. Create IAM Role in Child Accounts

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "budgets:DescribeBudgets",
      "budgets:DescribeNotificationsForBudget",
      "budgets:DescribeSubscribersForNotification"
    ],
    "Resource": "*"
  }]
}
```

### 2. Add Trust Relationship

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      "AWS": "arn:aws:iam::MANAGEMENT-ACCOUNT-ID:root"
    },
    "Action": "sts:AssumeRole"
  }]
}
```

### 3. Run with Role Assumption

```bash
# Now the tool can access AWS Budgets in each child account
./bud --assume-role-name BudgetReadRole
```

> **Note**: Without role assumption, the tool can only see AWS Budgets in the account where you're authenticated. If your AWS Budgets are in child accounts, you'll see "UNKNOWN" in the Adjustment column.

## Required IAM Permissions

### Management Account

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "organizations:ListAccounts",
      "ce:GetCostAndUsage",
      "budgets:DescribeBudgets",
      "budgets:DescribeNotificationsForBudget",
      "budgets:DescribeSubscribersForNotification",
      "sts:AssumeRole"
    ],
    "Resource": "*"
  }]
}
```

## Development

### Prerequisites

- Go 1.23 or higher
- AWS credentials configured
- Access to an AWS Organization

### Building from Source

```bash
git clone https://github.com/yourusername/bud.git
cd bud
go mod download
go build -o bud ./cmd/bud
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Project Structure

```
bud/
├── cmd/bud/    # CLI entry point
├── internal/
│   ├── analyzer/                # Spending analysis
│   ├── budgets/                 # AWS Budgets client
│   ├── cmd/                     # Cobra commands
│   ├── costexplorer/            # Cost Explorer client
│   ├── recommender/             # Recommendation engine
│   └── reporter/                # Report generation
└── pkg/types/                   # Shared types
```

## Troubleshooting

### "UNKNOWN" appears for all accounts

**Solution**: Use `--assume-role-name` flag with a role that has budget read permissions

### Rate limiting errors

**Solution**: Reduce concurrency with `--concurrency 3`

## Contributing

Contributions are welcome! Please submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)
- CLI powered by [Cobra](https://github.com/spf13/cobra)

---

## FAQ

### What's the difference between this and AWS Cost Explorer?

- **AWS Cost Explorer** shows you what you've spent
- **AWS Budgets** lets you set spending limits and get alerts
- **This tool** analyzes Cost Explorer data and recommends optimal AWS Budget configurations

### Does this tool modify my AWS Budgets?

No. This tool only **reads** data and **generates recommendations**. You must manually update your AWS Budgets based on the recommendations.

### Can I use this with Terraform/Pulumi/CloudFormation?

Yes! Export recommendations to JSON (`--output-format json`) and use them to update your Infrastructure as Code:

```bash
# Export to JSON
./bud --output-format json --output-file budgets.json

# Use in your IaC tool to update AWS Budget resources
```

### What if I don't have AWS Budgets configured yet?

The tool will show "NEW" in the Adjustment column and recommend initial budget amounts based on your spending patterns.

### When should I use OU filtering?

Use `--organizational-units` when you want to:
- **Analyze by team/department** - Different OUs for different teams
- **Different budget policies** - Production OUs may need different growth buffers than dev/test
- **Phased rollout** - Test on one OU before rolling out organization-wide
- **Compliance requirements** - Some OUs may have stricter budget controls
- **Performance** - Analyze smaller subsets faster

Example OU structure:
```
Root
├── ou-prod-12345678 (Production)
│   ├── Account A (prod workloads)
│   └── Account B (prod databases)
├── ou-dev-87654321 (Development)
│   ├── Account C (dev environment)
│   └── Account D (test environment)
└── ou-shared-11111111 (Shared Services)
    └── Account E (logging, monitoring)
```

Analyze only production accounts:
```bash
./bud --organizational-units ou-prod-12345678 --growth-buffer 15
```

---

**Note**: This tool provides recommendations only. Always review suggestions before updating your AWS Budgets in production.
