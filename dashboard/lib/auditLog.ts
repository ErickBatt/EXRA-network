/**
 * Audit Logging for Admin Operations
 * 
 * Logs all admin actions for security and compliance purposes.
 */

export interface AuditLogEntry {
  timestamp: string;
  adminEmail: string;
  action: string;
  details: Record<string, any>;
  status: 'success' | 'failure';
  errorMessage?: string;
}

// In-memory audit log (in production, write to database)
const auditLogs: AuditLogEntry[] = [];

/**
 * Log an admin action
 */
export function logAdminAction(
  adminEmail: string,
  action: string,
  details: Record<string, any>,
  status: 'success' | 'failure' = 'success',
  errorMessage?: string
): void {
  const entry: AuditLogEntry = {
    timestamp: new Date().toISOString(),
    adminEmail,
    action,
    details,
    status,
    errorMessage,
  };

  auditLogs.push(entry);

  // Log to console for debugging
  console.log(`[AUDIT] ${action} by ${adminEmail}: ${status}`, details);

  // In production, you would:
  // 1. Send to database
  // 2. Send to logging service (e.g., Sentry, DataDog)
  // 3. Alert on suspicious activity
}

/**
 * Get audit logs (for admin dashboard)
 */
export function getAuditLogs(limit: number = 100): AuditLogEntry[] {
  return auditLogs.slice(-limit).reverse();
}

/**
 * Clear audit logs (for testing)
 */
export function clearAuditLogs(): void {
  auditLogs.length = 0;
}

/**
 * Get audit logs for a specific admin
 */
export function getAuditLogsForAdmin(adminEmail: string, limit: number = 50): AuditLogEntry[] {
  return auditLogs
    .filter(log => log.adminEmail === adminEmail)
    .slice(-limit)
    .reverse();
}

/**
 * Get failed audit logs (potential security incidents)
 */
export function getFailedAuditLogs(limit: number = 50): AuditLogEntry[] {
  return auditLogs
    .filter(log => log.status === 'failure')
    .slice(-limit)
    .reverse();
}
