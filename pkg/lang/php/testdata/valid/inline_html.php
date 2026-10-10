<html>
  <body>before php</body>
  <?php $inHtml = true; ?>
  <p>between tags <?= $shortEcho ?> and <?= htmlspecialchars($v), $w ?> text</p>
  <?php if ($show): ?>
    <span>alternative syntax inside html</span>
  <?php endif; ?>
  <?php
  $x = 1 ?>
  trailing statement without semicolon
</html>
plain trailing content
