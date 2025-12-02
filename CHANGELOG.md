# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0-rc.2] - 2025-12-02

### Added
- Scoop package manager support for Windows users
- Comprehensive security scanning (CodeQL, gosec, govulncheck)
- Dependabot for automated dependency updates
- Codecov integration for test coverage tracking
- Coverage badge in README

### Changed
- Updated installation documentation with all platform options
- Improved Windows installation instructions

## [1.0.0-rc.1] - 2025-12-02

### Added
- Initial release of Bud
- Multi-account AWS spending analysis via Cost Explorer API
- AWS Budgets comparison and recommendations
- Policy-based budget configuration (OU, tag, and account-level policies)
- Cross-account role assumption for budget access
- Concurrent processing with configurable worker pools
- Progress indicators and color-coded output
- JSON and table output formats
- Organizational Unit (OU) filtering
- Account filtering by ID
- Configurable analysis period (months)
- Configurable growth buffer percentage
- Configurable minimum budget amounts
- Configurable rounding increments
- Retry logic with exponential backoff
- Graceful shutdown handling
- Comprehensive test coverage (44-100% across packages)
- Complete documentation (README, examples, testing guide)

### Features

#### Analysis Engine
- Historical spending analysis from AWS Cost Explorer
- Peak and average monthly spend calculation
- Spending trend analysis
- Multi-month analysis support (configurable)

#### Policy System
- OU-based policies for organizational structure
- Tag-based policies for flexible categorization
- Account-specific policy overrides
- Policy inheritance and priority resolution
- Policy validation on startup

#### Budget Recommendations
- Data-driven budget recommendations
- Growth buffer configuration
- Minimum budget enforcement
- Configurable rounding increments
- Priority flagging (HIGH/MEDIUM/LOW)
- Adjustment percentage calculation

#### Cross-Account Support
- AWS Organizations integration
- Automatic account discovery
- Cross-account role assumption
- Budget access in child accounts
- OU membership resolution
- Account tag loading

#### Output & Reporting
- Table format with color coding
- JSON export for automation
- Progress indicators
- Policy transparency in reports
- Clear adjustment indicators (NEW/UNKNOWN/±X%)

#### Configuration
- YAML configuration file support
- Command-line flag overrides
- AWS profile support
- Environment variable support
- Example configuration included

### Documentation
- Comprehensive README with examples
- Configuration guide with policy examples
- Testing guide
- Alternatives comparison document
- Contributing guidelines
- Code of conduct
- Security policy

### Technical
- Built with Go 1.23
- AWS SDK for Go v2
- Cobra CLI framework
- Viper configuration management
- Concurrent processing with worker pools
- Exponential backoff retry logic
- Graceful shutdown support

[Unreleased]: https://github.com/mskutin/bud/compare/v1.0.0-rc.2...HEAD
[1.0.0-rc.2]: https://github.com/mskutin/bud/compare/v1.0.0-rc.1...v1.0.0-rc.2
[1.0.0-rc.1]: https://github.com/mskutin/bud/releases/tag/v1.0.0-rc.1
