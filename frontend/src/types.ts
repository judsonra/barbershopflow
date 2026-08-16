export type User = {
  id: string; name: string; email?: string; phone?: string
  role: 'manager' | 'professional' | 'client' | 'superadmin'; professional_id?: string; customer_id?: string; active: boolean
}
export type MembershipOption = { membership_id: string; tenant_id: string; tenant_name: string; tenant_slug: string; role: string }
export type Tenant = {
  id: string; name: string; slug: string
  self_scheduling_enabled: boolean; auto_confirm_appointments: boolean; active: boolean; created_at: string
}
export type Service = { id: string; name: string; duration_minutes: number; price_cents: number; active: boolean }
export type Professional = { id: string; name: string; phone?: string; email?: string; cpf?: string; active: boolean }
export type ScheduleEntry = { weekday: number; start_minute: number; end_minute: number }
export type TimeOff = { id: string; professional_id: string; starts_at: string; ends_at: string; reason?: string }
export type Customer = { id: string; name: string; phone?: string; email?: string; active: boolean }
export type AdminCustomerMatch = Customer & { tenant_id: string; tenant_name: string; tenant_slug: string }
export type Appointment = {
  id: string; customer_id: string; customer_name: string; professional_id: string;
  professional_name: string; service_id: string; service_name: string; starts_at: string;
  ends_at: string; status: 'scheduled' | 'confirmed' | 'completed' | 'cancelled';
  notes?: string; price_cents: number
}
export type ProfessionalOccupancy = { professional_id: string; professional_name: string; available_minutes: number; booked_minutes: number; rate: number }
export type ProfessionalRevenue = { professional_id: string; professional_name: string; total_cents: number }
export type Report = {
  from: string; to: string
  occupancy: { overall_rate: number; by_professional: ProfessionalOccupancy[] }
  revenue: { total_cents: number; by_professional: ProfessionalRevenue[] }
  retention: { total_customers: number; returning_customers: number; rate: number }
}
