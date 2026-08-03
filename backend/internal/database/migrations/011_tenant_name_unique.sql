-- Nome da barbearia passa a ser unico (nao so o slug derivado dele), para
-- a checagem de disponibilidade em tempo real do cadastro fazer sentido.
CREATE UNIQUE INDEX tenants_name_idx ON tenants (lower(name));
