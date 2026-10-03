/**
 * HeldDialog is a dialog its caller holds, as a menu's item does, which no
 * trigger opens: whether it is open, the change the dialog asks for, and,
 * once closed, whether what it asked went through. The caller then moves
 * the focus, which no trigger takes back.
 */
export type HeldDialog = { open: boolean; onOpenChange: (open: boolean) => void; onClosed: (done: boolean) => void };
