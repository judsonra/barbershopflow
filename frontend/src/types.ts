export type Service = { id: string; name: string; duration_minutes: number; price_cents: number; active: boolean }
export type Professional = { id: string; name: string; phone?: string; active: boolean }
export type Customer = { id: string; name: string; phone?: string; email?: string }
export type Appointment = {
  id: string; customer_id: string; customer_name: string; professional_id: string;
  professional_name: string; service_id: string; service_name: string; starts_at: string;
  ends_at: string; status: 'scheduled' | 'confirmed' | 'completed' | 'cancelled';
  notes?: string; price_cents: number
}
