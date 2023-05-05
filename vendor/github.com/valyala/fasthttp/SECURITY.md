### TL;DR

We use a simplified version of [Golang Security Policy](https://go.dev/security).
For example, for now we skip CVE assignment.

### Reporting a Security Bug

Please report to us any issues you find. This document explains how to do that and what to expect in return.

All security bugs in our releases should be reported by email to oss-security@highload.solutions.
This mail is delivered to a small security team.
Your email will be acknowledged within 24 hours, and you'll receive a more detailed response
to your email within 72 hours indicating the next steps in handling your report.
For critical problems, you can encrypt your report using our PGP key (listed below).

Please use a descriptive subject line for your report email.
After the initial reply to your report, the security team will
endeavor to keep you informed of the progress being made towards a fix and full announcement.
These updates will be sent at least every five days.
In reality, this is more likely to be every 24-48 hours.

If you have not received a reply to your email within 48 hours or you have not heard from the security
team for the past five days please contact us by email to developers@highload.solutions or by Telegram message
to [our support](https://t.me/highload_support).
Please note that developers@highload.solutions list includes all developers, who may be outside our opensource security team.
When escalating on this list, please do not disclose the details of the issue.
Simply state that you're trying to reach a member of the security team.

### Flagging Existing Issues as Security-related

If you believe that an existing issue is security-related, we ask that you send an email to oss-security@highload.solutions.
The email should include the issue ID and a short description of why it should be handled according to this security policy.

### Disclosure Process

Our project uses the following disclosure process:

- Once the security report is received it is assigned a primary handler. This person coordinates the fix and release process.
- The issue is confirmed and a list of affected software is determined.
- Code is audited to find any potential similar problems.
- Fixes are prepared for the two most recent major releases and the head/master revision. These fixes are not yet committed to the public repository.
- To notify users, a new issue without security details is submitted to our GitHub repository.
- Three working days following this notification, the fixes are applied to the public repository and a new release is issued.
- On the date that the fixes are applied, announcement is published in the issue.

This process can take some time, especially when coordination is required with maintainers of other projects.
Every effort will be made to handle the bug in as timely a manner as possible, however it's important that we follow
the process described above to ensure that disclosures are handled consistently.

### Receiving Security Updates
The best way to receive security announcements is to subscribe ("Watch") to our repository.
Any GitHub issues pertaining to a security issue will be prefixed with [security].

### Comments on This Policy
If you have any suggestions to improve this policy, please send an email to oss-security@highload.solutions for discussion.

### PGP Key for oss-security@highload.ltd

We accept PGP-encrypted email, but the majority of the security team are not regular PGP users
so it's somewhat inconvenient. Please only use PGP for critical security reports.

```
-----BEGIN PGP PUBLIC KEY BLOCK-----

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
=lnt2
-----END PGP PUBLIC KEY BLOCK-----
```
