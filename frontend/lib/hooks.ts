"use client";

import useSWR, { mutate, type SWRConfiguration } from "swr";
import {
  getStatus,
  listInbounds,
  listInboundTypes,
  listOutbounds,
  listUsers,
  getConfig,
  getLiveTraffic,
} from "@/lib/api";

// Stable cache keys, one per GET endpoint. Mutations revalidate by these.
export const KEYS = {
  status: "status",
  inbounds: "inbounds",
  inboundTypes: "inbound-types",
  outbounds: "outbounds",
  users: "users",
  config: "config",
  liveTraffic: "live-traffic",
} as const;

// dedupingInterval:0 keeps the cache (instant render on revisit + background
// revalidate) while ensuring every mount actually refetches — this also keeps
// the test suite deterministic across re-renders.
const common: SWRConfiguration = { revalidateOnFocus: false, dedupingInterval: 0 };

export function useStatus() {
  return useSWR(KEYS.status, getStatus, common);
}
export function useInbounds() {
  return useSWR(KEYS.inbounds, listInbounds, common);
}
export function useInboundTypes() {
  return useSWR(KEYS.inboundTypes, listInboundTypes, common);
}
export function useOutbounds() {
  return useSWR(KEYS.outbounds, listOutbounds, common);
}
export function useUsers() {
  return useSWR(KEYS.users, listUsers, common);
}
export function useConfig() {
  return useSWR(KEYS.config, getConfig, common);
}
export function useLiveTraffic() {
  return useSWR(KEYS.liveTraffic, getLiveTraffic, { ...common, refreshInterval: 3000 });
}

// revalidate revalidates one cache key from anywhere (used after mutations).
export function revalidate(key: string) {
  return mutate(key);
}
