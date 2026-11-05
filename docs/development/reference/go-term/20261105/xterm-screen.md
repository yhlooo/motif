# XTerm Control Sequences（原文节选）

来源：https://invisible-island.net/xterm/ctlseqs/ctlseqs.html

采集日期：2026-11-05。

## The Alternate Screen Buffer

    XTerm maintains two screen buffers.  The Normal Screen Buffer allows you
    to scroll back to view saved lines of output up to the maximum set by
    the saveLines resource.  The Alternate Screen Buffer is exactly as large
    as the display, contains no additional saved lines.  When the Alternate
    Screen Buffer is active, you cannot scroll back to view saved lines.
    XTerm provides control sequences and menu entries for switching between
    the two.

## Select Graphic Rendition（原文节选）

来源：https://www.invisible-island.net/xterm/ctlseqs/ctlseqs.html

    Ps = 0  ⇒ Normal (default), VT100.
    Ps = 1  ⇒ Bold, VT100.
    Ps = 2  ⇒ Faint, decreased intensity, ECMA-48 2nd.
    Ps = 3 1  ⇒ Set foreground color to Red.
    Ps = 3 2  ⇒ Set foreground color to Green.
    Ps = 3 3  ⇒ Set foreground color to Yellow.
    Ps = 3 6  ⇒ Set foreground color to Cyan.
