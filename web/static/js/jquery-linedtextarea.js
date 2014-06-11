/**
 * Adapted from jQuery Lined Textarea Plugin
 * http://alan.blog-city.com/jquerylinedtextarea.htm
 *
 * Released under the MIT License:
 * http://www.opensource.org/licenses/mit-license.php
 */
(function($) {
	$.fn.linedtextarea = function() {
		/*
		 * Helper function to make sure the line numbers are always kept up to
		 * the current system
		 */
		var fillOutLines = function(linesDiv, h, lineNo) {
			while (linesDiv.height() <= h) {
				linesDiv.append("<div>" + lineNo + "</div>");
				lineNo++;
			}
			return lineNo;
		};

		return this.each(function() {
			var lineNo = 1;
			var textarea = $(this);

			/* Wrap the text area in the elements we need */
			textarea.wrap("<div class='linedtextarea' style='height:100%; overflow:hidden'></div>");
			textarea.css("width", "97%");
			textarea.parent().prepend("<div class='lines' style='width:3%'></div>");
			var linesDiv = textarea.parent().find(".lines");

			var scroll = function(tn) {
				var domTextArea = $(this)[0];
				var scrollTop = domTextArea.scrollTop;
				var clientHeight = domTextArea.clientHeight;
				linesDiv.css({
					'margin-top' : (-scrollTop) + "px"
				});
				lineNo = fillOutLines(linesDiv, scrollTop + clientHeight,
						lineNo);
			};
			/* React to the scroll event */
			textarea.scroll(scroll);
			$(window).resize(function() { textarea.scroll(); });
			/* We call scroll once to add the line numbers */
			textarea.scroll();
		});
	};
})(jQuery);
