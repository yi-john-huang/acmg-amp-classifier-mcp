# Security Policy

## Overview

The ACMG-AMP MCP Server is a medical genetics software system that handles sensitive genetic variant data. This document outlines our security practices and procedures for maintaining the highest standards of data protection and clinical safety.

## Security Standards

### Medical Software Compliance
- **HIPAA Considerations**: While this system doesn't directly handle PHI, it's designed with HIPAA-compliant practices
- **Clinical Audit Requirements**: Complete audit trails for all operations
- **Data Integrity**: Cryptographic verification of critical data
- **Access Control**: API key-based authentication with rate limiting

### Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Email security concerns to: [security@your-domain.com]
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact assessment
   - Suggested remediation (if any)

### Response Timeline

- **Initial Response**: Within 24 hours
- **Vulnerability Assessment**: Within 72 hours
- **Fix Development**: Within 7 days for critical issues
- **Patch Release**: Within 14 days for critical issues

### Severity Classification

#### Critical (24-48 hours)
- Remote code execution
- Authentication bypass
- Data exposure of genetic variants
- SQL injection vulnerabilities

#### High (3-7 days)
- Privilege escalation
- Cross-site scripting (XSS)
- Insecure direct object references
- Cryptographic vulnerabilities

#### Medium (7-14 days)
- Information disclosure
- Cross-site request forgery (CSRF)
- Insecure configurations
- Rate limiting bypass

#### Low (14-30 days)
- Security misconfigurations
- Verbose error messages
- Missing security headers

## Security Best Practices

### For Developers

1. **Never commit secrets to version control**
   - Use environment variables for all credentials
   - Utilize `.env.example` for configuration templates
   - Implement proper secret management in production

2. **Input Validation**
   - Validate all HGVS notation inputs
   - Sanitize user inputs to prevent injection attacks
   - Use parameterized queries for database operations

3. **Authentication & Authorization**
   - Implement API key rotation mechanisms
   - Use strong, unique API keys for each client
   - Apply rate limiting to prevent abuse

4. **Audit Logging**
   - Log all security-relevant events
   - Include correlation IDs for request tracing
   - Implement immutable audit trails

### For Deployment

1. **Environment Security**
   ```bash
   # Use Docker secrets in production
   docker-compose -f docker-compose.prod.yml up -d
   
   # Set up secrets using the provided script
   ./scripts/setup-secrets.sh
   ```

2. **Network Security**
   - Use TLS/HTTPS in production
   - Implement proper firewall rules
   - Restrict database access to application servers only

3. **Container Security**
   - Use non-root users in containers
   - Scan images for vulnerabilities
   - Keep base images updated

4. **Database Security**
   - Enable SSL/TLS for database connections
   - Use strong, unique passwords
   - Implement connection pooling limits
   - Regular security updates

## Security Features

### Built-in Security Controls

1. **Request Security**
   - Correlation ID tracking for audit trails
   - Request timeout protection
   - Security headers middleware
   - CORS protection

2. **Data Protection**
   - Parameterized database queries
   - Input validation and sanitization
   - Secure password generation for tests
   - Environment-based configuration

3. **Monitoring & Logging**
   - Structured JSON logging
   - Security event tracking
   - Request/response logging
   - Error tracking with correlation IDs

### Security Headers

The application automatically sets the following security headers:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

## Incident Response

### In Case of Security Incident

1. **Immediate Actions**
   - Isolate affected systems
   - Preserve evidence and logs
   - Notify security team immediately

2. **Assessment**
   - Determine scope and impact
   - Identify root cause
   - Document timeline of events

3. **Remediation**
   - Apply security patches
   - Update configurations
   - Rotate compromised credentials

4. **Recovery**
   - Restore services safely
   - Monitor for additional threats
   - Update security measures

### Post-Incident

1. **Documentation**
   - Complete incident report
   - Lessons learned analysis
   - Update security procedures

2. **Communication**
   - Notify affected users (if applicable)
   - Update security documentation
   - Share findings with development team

## Security Testing

### Automated Security Scanning

The project includes automated security scanning:

```bash
# Run security pre-commit hook
git commit  # Automatically triggers security scan

# Manual security scan
./scripts/security-scan.sh
```

### Manual Security Testing

1. **Code Review Checklist**
   - [ ] No hardcoded credentials
   - [ ] Proper input validation
   - [ ] Secure error handling
   - [ ] Audit logging implemented
   - [ ] Authentication mechanisms tested

2. **Penetration Testing**
   - Regular security assessments
   - Third-party security audits
   - Vulnerability scanning

## AI Tool Security Guidelines

### Using Kiro and Claude Code Safely

This project integrates with AI development tools (Kiro, Claude Code) that require special security considerations when handling sensitive genetic data and credentials.

#### Credential Protection for AI Tools

1. **File Access Control**
   ```bash
   # AI tools respect these ignore patterns
   .kiro-ignore         # Kiro AI tool ignore patterns
   .claudecode-ignore   # Claude Code specific patterns
   .dockerignore        # Docker build context protection
   .gitignore           # Version control protection
   ```

2. **Sensitive Data Exclusion**
   - **Medical Data**: Patient information, genetic variants, clinical data
   - **API Credentials**: ClinVar, gnomAD, COSMIC API keys
   - **Database Secrets**: Connection strings, passwords, JWT tokens
   - **Certificates**: SSL/TLS certificates and private keys
   - **Audit Trails**: Security logs and compliance reports

3. **Best Practices with AI Tools**
   ```bash
   # Before using AI tools, verify ignore files are in place
   ls -la .kiro-ignore .claudecode-ignore
   
   # Review what files AI tools can access
   git ls-files | grep -E "\.(key|pem|crt|cert)$"
   
   # Ensure no credentials are accidentally exposed
   grep -r "password\|secret\|key" --exclude-dir=.git .
   ```

#### Medical Data Handling with AI Tools

1. **HIPAA Compliance Considerations**
   - AI tools should never access real patient data
   - Use synthetic test data for development assistance
   - Maintain audit trails of AI tool interactions with medical systems

2. **Genetic Data Protection**
   ```yaml
   # Example patterns in .claudecode-ignore
   patient*data*
   genetic*data*
   variant*data*.json
   *clinical*trial*
   *medical*record*
   ```

3. **Clinical Safety Requirements**
   - AI-generated medical code must undergo clinical review
   - ACMG/AMP rule implementations require medical validation
   - Never commit AI-generated clinical decision logic without expert review

#### AI Tool Configuration Security

1. **Conversation History Protection**
   ```bash
   # Ensure AI tool conversations are not persisted
   conversation-history/
   claude-conversations/
   chatgpt-sessions/
   ai-tool-logs/
   *.ai-session
   ```

2. **Environment Isolation**
   ```bash
   # Use separate environments for AI tool development
   export ENVIRONMENT=development
   export AI_TOOL_MODE=safe
   export MEDICAL_DATA_ACCESS=false
   ```

3. **Access Monitoring**
   - Log AI tool file access patterns
   - Monitor for credential exposure in AI conversations
   - Regular audits of AI tool permissions and access

#### Emergency Response for AI Tool Incidents

If sensitive data is accidentally exposed to AI tools:

1. **Immediate Actions**
   - Disconnect AI tool access immediately
   - Review conversation/session logs
   - Identify exposed credentials or medical data

2. **Remediation Steps**
   - Rotate all potentially exposed credentials
   - Update ignore files to prevent future exposure
   - Review AI tool configuration and permissions

3. **Prevention Measures**
   - Implement pre-commit hooks to scan for credentials
   - Regular training on AI tool security practices
   - Automated monitoring of ignore file compliance

## Contact Information

- **Security Team**: [security@your-domain.com]
- **Development Team**: [dev@your-domain.com]
- **Emergency Contact**: [emergency@your-domain.com]
- **Medical Safety Officer**: [medical-safety@your-domain.com]

## Acknowledgments

We appreciate the security research community's efforts in responsibly disclosing vulnerabilities. Contributors who report valid security issues will be acknowledged in our security advisories (with their permission).

---

**Last Updated**: January 2025
**Next Review**: July 2025