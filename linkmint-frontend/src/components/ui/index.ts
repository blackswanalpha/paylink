/**
 * The LinkMint component kit (work03 / frontendfeature.md §2.5) — one import surface for every screen.
 * Feature pages (work09–18) should import primitives from `@/components/ui` rather than deep paths, so
 * the kit can evolve behind this barrel. Existing foundation primitives are folded in here too.
 */

// --- Actions ---
export { Button, IconButton, GoldButton } from './Button';
export type { ButtonProps, IconButtonProps, GoldButtonProps } from './Button';

// --- Overlays ---
export { Modal } from './Modal';
export type { ModalProps, ModalSize } from './Modal';
export { Drawer } from './Drawer';
export type { DrawerProps, DrawerPlacement, DrawerSize } from './Drawer';
export { Menu } from './Menu';
export type { MenuProps, MenuAction, MenuGroup } from './Menu';
export { Tooltip } from './Tooltip';
export type { TooltipProps } from './Tooltip';

// --- Navigation & flow ---
export { Tabs } from './Tabs';
export type { TabsProps, TabItem } from './Tabs';
export { Stepper } from './Stepper';
export type { StepperProps, StepDescriptor } from './Stepper';

// --- Forms ---
export { FormField } from './FormField';
export type { FormFieldProps } from './FormField';

// --- Data display ---
export { DataTable } from './DataTable';
export type {
  DataTableProps,
  DataTableColumn,
  DataTableSort,
  SortDirection,
  ColumnAlign,
} from './DataTable';
export { CopyField } from './CopyField';
export type { CopyFieldProps } from './CopyField';
export { QRBlock } from './QRBlock';
export type { QRBlockProps } from './QRBlock';
export { Avatar } from './Avatar';
export type { AvatarProps } from './Avatar';

// --- Foundation primitives (shipped earlier; folded into the kit surface) ---
export { Panel } from './Panel';
export type { PanelProps } from './Panel';
export { PageHeader } from './PageHeader';
export type { PageHeaderProps } from './PageHeader';
export { MetricCard } from './MetricCard';
export type { MetricCardProps } from './MetricCard';
export { Sparkline } from './Sparkline';
export type { SparklineProps } from './Sparkline';
export { EmptyState } from './EmptyState';
export type { EmptyStateProps } from './EmptyState';
export { Skeleton, MetricCardSkeleton, TableRowsSkeleton } from './Skeleton';
export { AmountDisplay } from './AmountDisplay';
export type { AmountDisplayProps } from './AmountDisplay';
export { AddressChip } from './AddressChip';
export type { AddressChipProps } from './AddressChip';
export {
  StatusPill,
  PayLinkStatusPill,
  PaymentStatusPill,
  statusKindForPayLink,
  statusKindForPayment,
} from './StatusPill';
export type { StatusPillProps, StatusKind } from './StatusPill';

// --- Loading, empty & skeleton states (work06) ---
export { Loadable, AsyncBoundary } from './Loadable';
export type { LoadableProps, AsyncBoundaryProps } from './Loadable';
export {
  SkeletonRegion,
  MetricGridSkeleton,
  TableSkeleton,
  DetailPanelSkeleton,
  FormSkeleton,
  ListCardSkeleton,
} from './skeletons';
export type { SkeletonRegionProps } from './skeletons';
export {
  EMPTY_STATES,
  CatalogEmptyState,
  NoPayLinksEmpty,
  NoPaymentsEmpty,
  NoSearchResultsEmpty,
  NoApiKeysEmpty,
} from './emptyStates';
export type { EmptySurface, CatalogEmptyStateProps } from './emptyStates';

// --- Live in src/components/ (re-exported so the kit has a single import surface) ---
export { KeyValueRow } from '../KeyValueRow';
export type { KeyValueRowProps } from '../KeyValueRow';

// --- Error & feedback system (work04) ---
export { ErrorBanner } from '../ErrorBanner';
export type { ErrorBannerProps } from '../ErrorBanner';
export { ErrorBoundary } from '../ErrorBoundary';
export { ErrorFallback } from '../ErrorFallback';
export type { ErrorFallbackProps } from '../ErrorFallback';
export { KycGate } from '../KycGate';
export type { KycGateProps } from '../KycGate';
