require "eth"

def generate_key_variants(known_part)
  # Возможные символы для каждого недостающего символа
  hex_chars = '0123456789abcdef'.chars

  # Количество недостающих символов
  missing_length = 5

  # Массив для хранения всех сгенерированных вариантов
  all_variants = []

  # Перебираем все возможные способы разделения 5 символов между началом и концом
  (0..missing_length).each do |i|
    j = missing_length - i # количество символов в конце

    # Генерируем все возможные комбинации символов для начала и конца
    start_combinations = hex_chars.repeated_permutation(i)
    end_combinations = hex_chars.repeated_permutation(j)

    # Создаем ключи, добавляя известную часть между началом и концом
    start_combinations.each do |start|
      end_combinations.each do |end_part|
        private = start.join + known_part + end_part.join
        variant =  "#{ private } -> #{ Eth::Key.new(priv: private).address }"
        all_variants << variant
        puts variant
      end
    end
  end

  all_variants
end

known_part = "0a5c2dffb9a6e1240e7d8f58b1e68d6c9fce1e6e9b0a5c0e7f1b2c3a4b5"
generate_key_variants known_part