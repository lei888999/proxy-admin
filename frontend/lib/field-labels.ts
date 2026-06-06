// Human-readable labels for inbound publicInfo keys, so the UI shows
// "Reality 公钥" instead of the raw "realityPublicKey" field name.
const LABELS: Record<string, string> = {
  flow: "Flow",
  handshake: "握手域名",
  handshakePort: "握手端口",
  realityPublicKey: "Reality 公钥",
  serverName: "SNI 域名",
  shortId: "Short ID",
  upMbps: "上行 (Mbps)",
  downMbps: "下行 (Mbps)",
  insecure: "跳过证书校验",
  password: "密码",
};

// fieldLabel returns a friendly label for a publicInfo key, falling back to a
// spaced Title-case form of the raw camelCase key when unmapped.
export function fieldLabel(key: string): string {
  if (LABELS[key]) return LABELS[key];
  const spaced = key.replace(/([a-z])([A-Z])/g, "$1 $2");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}
