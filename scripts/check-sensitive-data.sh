#!/bin/bash
# Script to check for sensitive data before open sourcing

set -e

echo "🔍 Checking for sensitive data..."
echo ""

# Colors
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

ISSUES_FOUND=0

# Check for AWS account IDs (12-digit numbers)
echo "Checking for AWS account IDs..."
if grep -r -E "[0-9]{12}" . --exclude-dir=.git --exclude-dir=vendor --exclude="*.sh" --exclude="go.sum" | grep -v "example" | grep -v "123456789012"; then
    echo -e "${RED}⚠️  Found potential AWS account IDs${NC}"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✓ No AWS account IDs found${NC}"
fi
echo ""

# Check for "mskutin" references that should be generic
echo "Checking for internal references..."
if grep -r "mskutin" . --exclude-dir=.git --exclude-dir=vendor --exclude="*.sh" --exclude="go.mod" --exclude="LICENSE" | grep -v "github.com/mskutin"; then
    echo -e "${YELLOW}⚠️  Found 'mskutin' references - review if these should be generic${NC}"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✓ No problematic internal references${NC}"
fi
echo ""

# Check for email addresses
echo "Checking for email addresses..."
if grep -r -E "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}" . --exclude-dir=.git --exclude-dir=vendor --exclude="*.sh" --exclude="*.md" | grep -v "example.com"; then
    echo -e "${RED}⚠️  Found email addresses${NC}"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✓ No email addresses found${NC}"
fi
echo ""

# Check for AWS access keys (starts with AKIA)
echo "Checking for AWS access keys..."
if grep -r "AKIA[0-9A-Z]{16}" . --exclude-dir=.git --exclude-dir=vendor --exclude="*.sh"; then
    echo -e "${RED}⚠️  Found potential AWS access keys${NC}"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
    echo -e "${GREEN}✓ No AWS access keys found${NC}"
fi
echo ""

# Check for AWS secret keys (40 characters base64)
echo "Checking for AWS secret keys..."
if grep -r -E "[A-Za-z0-9/+=]{40}" . --exclude-dir=.git --exclude-dir=vendor --exclude="*.sh" --exclude="go.sum" | head -5; then
    echo -e "${YELLOW}⚠️  Found potential secrets - manual review needed${NC}"
fi
echo ""

# Check for TODO/FIXME comments
echo "Checking for TODO/FIXME comments..."
if grep -r -E "(TODO|FIXME|XXX|HACK)" . --exclude-dir=.git --exclude-dir=vendor --exclude="*.sh" --include="*.go"; then
    echo -e "${YELLOW}⚠️  Found TODO/FIXME comments - review before release${NC}"
fi
echo ""

# Check for debug/test files
echo "Checking for debug/test files..."
if find . -name "*.json" -not -path "*/vendor/*" -not -path "*/.git/*" -not -path "*/testdata/*"; then
    echo -e "${YELLOW}⚠️  Found JSON files - ensure they don't contain real data${NC}"
fi
echo ""

# Summary
echo "================================"
if [ $ISSUES_FOUND -eq 0 ]; then
    echo -e "${GREEN}✓ No critical issues found!${NC}"
    echo "Manual review still recommended before open sourcing."
else
    echo -e "${RED}⚠️  Found $ISSUES_FOUND potential issues${NC}"
    echo "Please review the findings above before open sourcing."
fi
echo "================================"

exit $ISSUES_FOUND
